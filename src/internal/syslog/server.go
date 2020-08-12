package syslog

import (
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"time"

	"code.cloudfoundry.org/go-loggregator"
	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	metrics "code.cloudfoundry.org/go-metric-registry"
	"code.cloudfoundry.org/tlsconfig"
	"github.com/influxdata/go-syslog/v2"
	"github.com/influxdata/go-syslog/v2/octetcounting"

	"net"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/net/context"
)

type Server struct {
	sync.Mutex
	port        int
	l           net.Listener
	envelopes   chan *loggregator_v2.Envelope
	syslogCert  string
	syslogKey   string
	idleTimeout time.Duration

	ingress        metrics.Counter
	invalidIngress metrics.Counter

	loggr *log.Logger
}

type MetricsRegistry interface {
	NewCounter(name, helpText string, opts ...metrics.MetricOption) metrics.Counter
}

type ServerOption func(s *Server)

func NewServer(
	loggr *log.Logger,
	m MetricsRegistry,
	opts ...ServerOption,
) *Server {
	s := &Server{
		loggr:       loggr,
		envelopes:   make(chan *loggregator_v2.Envelope, 100),
		idleTimeout: 2 * time.Minute,
	}

	for _, o := range opts {
		o(s)
	}

	s.ingress = m.NewCounter(
		"ingress",
		"Total syslog messages ingressed successfully.",
	)
	s.invalidIngress = m.NewCounter(
		"invalid_ingress",
		"Total number of syslog messages unable to be converted to valid envelopes.",
	)

	return s
}

func WithServerPort(p int) ServerOption {
	return func(s *Server) {
		s.port = p
	}
}

func WithServerTLS(cert, key string) ServerOption {
	return func(s *Server) {
		s.syslogCert = cert
		s.syslogKey = key
	}
}

func WithIdleTimeout(d time.Duration) ServerOption {
	return func(s *Server) {
		s.idleTimeout = d
	}
}

func (s *Server) Start() {
	var l net.Listener
	var err error
	if s.syslogKey != "" || s.syslogCert != "" {
		tlsConfig := s.buildTLSConfig()
		l, err = tls.Listen("tcp", fmt.Sprintf(":%d", s.port), tlsConfig)
		if err != nil {
			s.loggr.Fatalf("unable to start syslog server: %s", err)
		}
	} else {
		l, err = net.Listen("tcp", fmt.Sprintf(":%d", s.port))
		if err != nil {
			s.loggr.Fatalf("unable to start syslog server: %s", err)
		}
	}
	defer s.Stop()

	s.Lock()
	s.l = l
	s.Unlock()

	for {
		c, err := l.Accept()
		if err != nil {
			s.loggr.Printf("syslog server no longer accepting connections: %s", err)
			return
		}
		go s.handleConnection(c)
	}
}

func (s *Server) Stream(ctx context.Context, req *loggregator_v2.EgressBatchRequest) loggregator.EnvelopeStream {
	return func() []*loggregator_v2.Envelope {
		select {
		case envelope := <-s.envelopes:
			return []*loggregator_v2.Envelope{envelope}
		case <-ctx.Done():
			return []*loggregator_v2.Envelope{}
		}
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()
	s.setReadDeadline(conn)

	p := octetcounting.NewParser()
	p.WithListener(s.parseListenerForConnection(conn))
	p.Parse(conn)
}

func (s *Server) parseListenerForConnection(conn net.Conn) syslog.ParserListener {
	return func(res *syslog.Result) {
		s.parseListener(res)
		s.setReadDeadline(conn)
	}
}

func (s *Server) setReadDeadline(conn net.Conn) {
	err := conn.SetReadDeadline(time.Now().Add(s.idleTimeout))
	if err != nil {
		s.loggr.Printf("syslog server could not set deadline on connection: %s", err)
	}
}

func (s *Server) parseListener(res *syslog.Result) {
	if res.Error != nil {
		s.invalidIngress.Add(1)
		s.loggr.Printf("unable to parse syslog message: %s", res.Error)
		return
	}

	msg := res.Message
	env, err := s.convertToEnvelope(msg)
	if err != nil {
		s.invalidIngress.Add(1)
		s.loggr.Printf("unable to convert syslog message to envelope: %s", err)
		return
	}

	s.envelopes <- env

	s.ingress.Add(1)
}

func (s *Server) convertToEnvelope(msg syslog.Message) (*loggregator_v2.Envelope, error) {
	procID := msg.ProcID()
	if procID == nil {
		return nil, fmt.Errorf("missing proc ID in syslog message")
	}

	instanceId := s.instanceIdFromPID(*procID)
	sourceID := msg.Appname()
	if sourceID == nil {
		return nil, fmt.Errorf("missing app name in syslog message")
	}

	env := &loggregator_v2.Envelope{
		SourceId:   *sourceID,
		Timestamp:  msg.Timestamp().UnixNano(),
		InstanceId: instanceId,
		Tags:       map[string]string{},
	}

	if msg.StructuredData() != nil {
		for envType, payload := range *msg.StructuredData() {
			var err error
			env, err = convertStructuredData(env, envType, payload)
			if err != nil {
				return nil, err
			}
		}
	}

	if env.GetMessage() == nil && msg.Message() != nil {
		env = s.convertMessage(env, msg)
	}

	if env.GetMessage() == nil {
		return nil, fmt.Errorf("missing message data in syslog message")
	}

	return env, nil
}

func (s *Server) convertMessage(env *loggregator_v2.Envelope, msg syslog.Message) *loggregator_v2.Envelope {
	env.Message = &loggregator_v2.Envelope_Log{
		Log: &loggregator_v2.Log{
			Payload: []byte(strings.TrimSpace(*msg.Message())),
			Type:    s.typeFromPriority(int(*msg.Priority())),
		},
	}

	return env
}

func convertStructuredData(env *loggregator_v2.Envelope, envType string, payload map[string]string) (*loggregator_v2.Envelope, error) {
	switch {
	case strings.HasPrefix(envType, "counter"):
		return convertCounter(env, payload)
	case strings.HasPrefix(envType, "gauge"):
		return convertGauge(env, payload)
	case strings.HasPrefix(envType, "event"):
		return convertEvent(env, payload)
	case strings.HasPrefix(envType, "timer"):
		return convertTimer(env, payload)
	case strings.HasPrefix(envType, "tags"):
		return convertTags(env, payload), nil
	default:
		return nil, fmt.Errorf(`unknown envelope type for structured data: [%s="%#v"]`, envType, payload)
	}
}

func convertTimer(env *loggregator_v2.Envelope, msg map[string]string) (*loggregator_v2.Envelope, error) {
	start, err := strconv.ParseInt(msg["start"], 10, 64)
	if err != nil {
		return nil, err
	}

	stop, err := strconv.ParseInt(msg["stop"], 10, 64)
	if err != nil {
		return nil, err
	}

	env.Message = &loggregator_v2.Envelope_Timer{
		Timer: &loggregator_v2.Timer{
			Name:  msg["name"],
			Start: start,
			Stop:  stop,
		},
	}

	return env, nil
}

func convertEvent(env *loggregator_v2.Envelope, msg map[string]string) (*loggregator_v2.Envelope, error) {
	env.Message = &loggregator_v2.Envelope_Event{
		Event: &loggregator_v2.Event{
			Title: msg["title"],
			Body:  msg["body"],
		},
	}

	return env, nil
}

func convertCounter(env *loggregator_v2.Envelope, msg map[string]string) (*loggregator_v2.Envelope, error) {
	delta, err := strconv.ParseUint(msg["delta"], 10, 64)
	if err != nil {
		return nil, err
	}
	total, err := strconv.ParseUint(msg["total"], 10, 64)
	if err != nil {
		return nil, err
	}
	env.Message = &loggregator_v2.Envelope_Counter{
		Counter: &loggregator_v2.Counter{
			Name:  msg["name"],
			Delta: delta,
			Total: total,
		},
	}
	return env, nil
}

func convertGauge(env *loggregator_v2.Envelope, msg map[string]string) (*loggregator_v2.Envelope, error) {
	unit, ok := msg["unit"]
	if !ok {
		return nil, errors.New("expected unit not found in gauge")
	}
	value, err := strconv.ParseFloat(msg["value"], 64)
	if err != nil {
		return nil, err
	}
	env.Message = &loggregator_v2.Envelope_Gauge{
		Gauge: &loggregator_v2.Gauge{
			Metrics: map[string]*loggregator_v2.GaugeValue{
				msg["name"]: {
					Unit:  unit,
					Value: value,
				},
			},
		},
	}
	return env, nil
}

func convertTags(env *loggregator_v2.Envelope, msg map[string]string) *loggregator_v2.Envelope {
	if env.Tags == nil {
		env.Tags = map[string]string{}
	}
	for k, v := range msg {
		env.Tags[k] = v
	}

	return env
}

func (s *Server) typeFromPriority(priority int) loggregator_v2.Log_Type {
	if priority == 11 {
		return loggregator_v2.Log_ERR
	}

	return loggregator_v2.Log_OUT
}

func (s *Server) instanceIdFromPID(pid string) string {
	pid = strings.Trim(pid, "[]")

	pidToks := strings.Split(pid, "/")

	return pidToks[len(pidToks)-1]
}

func (s *Server) Addr() string {
	s.Lock()
	defer s.Unlock()

	if s.l != nil && s.l.Addr() != nil {
		return s.l.Addr().String()
	}
	return ""
}

func (s *Server) Stop() {
	s.Lock()
	defer s.Unlock()

	if s.l != nil {
		s.l.Close()
		s.l = nil
	}
}

func (s *Server) buildTLSConfig() *tls.Config {
	tlsConfig, err := tlsconfig.Build(
		tlsconfig.WithInternalServiceDefaults(),
		tlsconfig.WithIdentityFromFile(s.syslogCert, s.syslogKey),
	).Server()

	if err != nil {
		log.Fatal(err)
	}
	return tlsConfig
}

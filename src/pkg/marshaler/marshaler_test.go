package marshaler_test

import (
	"bytes"
	"errors"
	"io"
	"strings"

	"code.cloudfoundry.org/go-log-cache/v2/rpc/logcache_v1"
	"code.cloudfoundry.org/log-cache/pkg/marshaler"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("PromqlMarshaler", func() {
	Context("Marshal()", func() {
		It("handles a scalar instant query result", func() {
			marshaler := marshaler.NewPromqlMarshaler(&mockMarshaler{})

			result, err := marshaler.Marshal(&logcache_v1.PromQL_InstantQueryResult{
				Result: &logcache_v1.PromQL_InstantQueryResult_Scalar{
					Scalar: &logcache_v1.PromQL_Scalar{
						Time:  "1.234",
						Value: 2.5,
					},
				},
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(MatchJSON(`{
				"status": "success",
				"data": {
					"resultType": "scalar",
					"result": [1.234, "2.5"]
				}
			}`))
			Expect(string(result)).To(MatchRegexp(`\n$`))
		})

		It("handles a vector instant query result", func() {
			marshaler := marshaler.NewPromqlMarshaler(&mockMarshaler{})

			result, err := marshaler.Marshal(&logcache_v1.PromQL_InstantQueryResult{
				Result: &logcache_v1.PromQL_InstantQueryResult_Vector{
					Vector: &logcache_v1.PromQL_Vector{
						Samples: []*logcache_v1.PromQL_Sample{
							{
								Metric: map[string]string{
									"deployment": "cf",
									"tag-name":   "tag-value",
								},
								Point: &logcache_v1.PromQL_Point{
									Time:  "1",
									Value: 2.5,
								},
							},
							{
								Metric: map[string]string{
									"deployment": "cf",
									"tag-name2":  "tag-value2",
								},
								Point: &logcache_v1.PromQL_Point{
									Time:  "2",
									Value: 3.5,
								},
							},
						},
					},
				},
			})

			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(MatchJSON(`{
				"status": "success",
				"data": {
					"resultType": "vector",
					"result": [
						{
							"metric": {
								"deployment": "cf",
								"tag-name":   "tag-value"
							},
							"value": [ 1.000, "2.5" ]
						},
						{
							"metric": {
								"deployment": "cf",
								"tag-name2":   "tag-value2"
							},
							"value": [ 2.000, "3.5" ]
						}
					]
				}
			}`))
			Expect(string(result)).To(MatchRegexp(`\n$`))
		})

		It("handles an empty vector instant query result", func() {
			marshaler := marshaler.NewPromqlMarshaler(&mockMarshaler{})

			result, err := marshaler.Marshal(&logcache_v1.PromQL_InstantQueryResult{
				Result: &logcache_v1.PromQL_InstantQueryResult_Vector{
					Vector: &logcache_v1.PromQL_Vector{
						Samples: []*logcache_v1.PromQL_Sample{},
					},
				},
			})

			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(MatchJSON(`{
				"status": "success",
				"data": {
					"resultType": "vector",
					"result": []
				}
			}`))
		})

		It("handles a vector instant query result with no tags", func() {
			marshaler := marshaler.NewPromqlMarshaler(&mockMarshaler{})

			result, err := marshaler.Marshal(&logcache_v1.PromQL_InstantQueryResult{
				Result: &logcache_v1.PromQL_InstantQueryResult_Vector{
					Vector: &logcache_v1.PromQL_Vector{
						Samples: []*logcache_v1.PromQL_Sample{
							{
								Point: &logcache_v1.PromQL_Point{
									Time:  "1",
									Value: 2.5,
								},
							},
						},
					},
				},
			})

			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(MatchJSON(`{
				"status": "success",
				"data": {
					"resultType": "vector",
					"result": [
						{
							"metric": {},
							"value": [ 1.000, "2.5" ]
						}
					]
				}
			}`))
		})

		It("handles a matrix instant query result", func() {
			marshaler := marshaler.NewPromqlMarshaler(&mockMarshaler{})

			result, err := marshaler.Marshal(&logcache_v1.PromQL_InstantQueryResult{
				Result: &logcache_v1.PromQL_InstantQueryResult_Matrix{
					Matrix: &logcache_v1.PromQL_Matrix{
						Series: []*logcache_v1.PromQL_Series{
							{
								Metric: map[string]string{
									"deployment": "cf",
									"tag-name":   "tag-value",
								},
								Points: []*logcache_v1.PromQL_Point{
									{
										Time:  "1",
										Value: 2.5,
									},
									{
										Time:  "2",
										Value: 3.5,
									},
								},
							},
							{
								Metric: map[string]string{
									"deployment": "cf",
									"tag-name2":  "tag-value2",
								},
								Points: []*logcache_v1.PromQL_Point{
									{
										Time:  "1",
										Value: 4.5,
									},
									{
										Time:  "2",
										Value: 6.5,
									},
								},
							},
						},
					},
				},
			})

			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(MatchJSON(`{
				"status": "success",
				"data": {
					"resultType": "matrix",
					"result": [
						{
							"metric": {
								"deployment": "cf",
								"tag-name":   "tag-value"
							},
							"values": [
								[ 1, "2.5" ],
								[ 2, "3.5" ]
							]
						},
						{
							"metric": {
								"deployment": "cf",
								"tag-name2":   "tag-value2"
							},
							"values": [
								[ 1, "4.5" ],
								[ 2, "6.5" ]
							]
						}
					]
				}
			}`))
			Expect(string(result)).To(MatchRegexp(`\n$`))
		})

		It("handles an empty matrix instant query result", func() {
			marshaler := marshaler.NewPromqlMarshaler(&mockMarshaler{})

			result, err := marshaler.Marshal(&logcache_v1.PromQL_InstantQueryResult{
				Result: &logcache_v1.PromQL_InstantQueryResult_Matrix{
					Matrix: &logcache_v1.PromQL_Matrix{
						Series: []*logcache_v1.PromQL_Series{},
					},
				},
			})

			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(MatchJSON(`{
				"status": "success",
				"data": {
					"resultType": "matrix",
					"result": []
				}
			}`))
		})

		It("handles a matrix instant query result with no tags", func() {
			marshaler := marshaler.NewPromqlMarshaler(&mockMarshaler{})

			result, err := marshaler.Marshal(&logcache_v1.PromQL_InstantQueryResult{
				Result: &logcache_v1.PromQL_InstantQueryResult_Matrix{
					Matrix: &logcache_v1.PromQL_Matrix{
						Series: []*logcache_v1.PromQL_Series{
							{
								Points: []*logcache_v1.PromQL_Point{
									{
										Time:  "1",
										Value: 2.5,
									},
									{
										Time:  "2",
										Value: 3.5,
									},
								},
							},
						},
					},
				},
			})

			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(MatchJSON(`{
				"status": "success",
				"data": {
					"resultType": "matrix",
					"result": [
						{
							"metric": {},
							"values": [
								[ 1, "2.5" ],
								[ 2, "3.5" ]
							]
						}
					]
				}
			}`))
		})

		It("handles a matrix range query result", func() {
			marshaler := marshaler.NewPromqlMarshaler(&mockMarshaler{})

			result, err := marshaler.Marshal(&logcache_v1.PromQL_RangeQueryResult{
				Result: &logcache_v1.PromQL_RangeQueryResult_Matrix{
					Matrix: &logcache_v1.PromQL_Matrix{
						Series: []*logcache_v1.PromQL_Series{
							{
								Metric: map[string]string{
									"deployment": "cf",
									"tag-name":   "tag-value",
								},
								Points: []*logcache_v1.PromQL_Point{
									{
										Time:  "1",
										Value: 2.5,
									},
									{
										Time:  "2",
										Value: 3.5,
									},
								},
							},
							{
								Metric: map[string]string{
									"deployment": "cf",
									"tag-name2":  "tag-value2",
								},
								Points: []*logcache_v1.PromQL_Point{
									{
										Time:  "1",
										Value: 4.5,
									},
									{
										Time:  "2",
										Value: 6.5,
									},
								},
							},
						},
					},
				},
			})

			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(MatchJSON(`{
				"status": "success",
				"data": {
					"resultType": "matrix",
					"result": [
						{
							"metric": {
								"deployment": "cf",
								"tag-name":   "tag-value"
							},
							"values": [
								[ 1, "2.5" ],
								[ 2, "3.5" ]
							]
						},
						{
							"metric": {
								"deployment": "cf",
								"tag-name2":   "tag-value2"
							},
							"values": [
								[ 1, "4.5" ],
								[ 2, "6.5" ]
							]
						}
					]
				}
			}`))
		})

		It("handles an empty matrix range query result", func() {
			marshaler := marshaler.NewPromqlMarshaler(&mockMarshaler{})

			result, err := marshaler.Marshal(&logcache_v1.PromQL_RangeQueryResult{
				Result: &logcache_v1.PromQL_RangeQueryResult_Matrix{
					Matrix: &logcache_v1.PromQL_Matrix{
						Series: []*logcache_v1.PromQL_Series{},
					},
				},
			})

			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(MatchJSON(`{
				"status": "success",
				"data": {
					"resultType": "matrix",
					"result": []
				}
			}`))
		})

		It("handles a matrix range query result with no tags", func() {
			marshaler := marshaler.NewPromqlMarshaler(&mockMarshaler{})

			result, err := marshaler.Marshal(&logcache_v1.PromQL_RangeQueryResult{
				Result: &logcache_v1.PromQL_RangeQueryResult_Matrix{
					Matrix: &logcache_v1.PromQL_Matrix{
						Series: []*logcache_v1.PromQL_Series{
							{
								Points: []*logcache_v1.PromQL_Point{
									{
										Time:  "1",
										Value: 2.5,
									},
									{
										Time:  "2",
										Value: 3.5,
									},
								},
							},
						},
					},
				},
			})

			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(MatchJSON(`{
				"status": "success",
				"data": {
					"resultType": "matrix",
					"result": [
						{
							"metric": {},
							"values": [
								[ 1, "2.5" ],
								[ 2, "3.5" ]
							]
						}
					]
				}
			}`))
		})

		It("reports errors for invalid timestamps", func() {
			marshaler := marshaler.NewPromqlMarshaler(&mockMarshaler{})

			_, err := marshaler.Marshal(&logcache_v1.PromQL_InstantQueryResult{
				Result: &logcache_v1.PromQL_InstantQueryResult_Scalar{
					Scalar: &logcache_v1.PromQL_Scalar{
						Time:  "potato",
						Value: 2.5,
					},
				},
			})
			Expect(err).To(HaveOccurred())
		})

		It("falls back to the fallback marshaler for non-PromQL replies", func() {
			marshaler := marshaler.NewPromqlMarshaler(&mockMarshaler{})

			result, err := marshaler.Marshal(nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal([]byte("mock marshaled result\n")))
		})

		It("passes through errors from the fallback marshaler", func() {
			marshaler := marshaler.NewPromqlMarshaler(&mockMarshaler{
				marshalError: errors.New("expected error"),
			})

			_, err := marshaler.Marshal(nil)
			Expect(err).To(HaveOccurred())
		})
	})

	Context("NewEncoder()", func() {
		It("can encode to a writer", func() {
			encoded := bytes.NewBuffer(nil)
			marshaler := marshaler.NewPromqlMarshaler(&mockMarshaler{})
			encoder := marshaler.NewEncoder(encoded)

			err := encoder.Encode(&logcache_v1.PromQL_InstantQueryResult{
				Result: &logcache_v1.PromQL_InstantQueryResult_Scalar{
					Scalar: &logcache_v1.PromQL_Scalar{
						Time:  "1",
						Value: 2.5,
					},
				},
			})

			Expect(err).ToNot(HaveOccurred())
			Expect(encoded.String()).To(MatchJSON(`{
				"status": "success",
				"data": {
					"resultType": "scalar",
					"result": [1, "2.5"]
				}
			}`))
		})

		It("falls back to the fallback marshaler for non-PromQL replies", func() {
			encoded := bytes.NewBuffer(nil)
			marshaler := marshaler.NewPromqlMarshaler(&mockMarshaler{})
			encoder := marshaler.NewEncoder(encoded)

			err := encoder.Encode(nil)

			Expect(err).ToNot(HaveOccurred())

			Expect(err).ToNot(HaveOccurred())
			Expect(encoded.String()).To(Equal("mock encoded result"))
		})

		It("passes through errors from the fallback marshaler", func() {
			encoded := bytes.NewBuffer(nil)
			marshaler := marshaler.NewPromqlMarshaler(&mockMarshaler{
				encodeError: errors.New("expected error"),
			})
			encoder := marshaler.NewEncoder(encoded)

			err := encoder.Encode(nil)

			Expect(err).To(HaveOccurred())
		})
	})

	Context("Unmarshal()", func() {
		It("handles a scalar instant query result", func() {
			marshaler := marshaler.NewPromqlMarshaler(&mockMarshaler{})

			var result logcache_v1.PromQL_InstantQueryResult
			err := marshaler.Unmarshal([]byte(`{
				"status": "success",
				"data": {
					"resultType": "scalar",
					"result": [1.777, "2.5"]
				}
			}`), &result)
			Expect(err).ToNot(HaveOccurred())

			Expect(result.GetScalar().GetTime()).To(Equal("1.777"))
			Expect(result.GetScalar().GetValue()).To(Equal(2.5))
		})

		It("handles a vector instant query result", func() {
			marshaler := marshaler.NewPromqlMarshaler(&mockMarshaler{})

			var result logcache_v1.PromQL_InstantQueryResult
			err := marshaler.Unmarshal([]byte(`{
				"status": "success",
				"data": {
					"resultType": "vector",
					"result": [
						{
							"metric": {
								"deployment": "cf",
								"tag-name":   "tag-value"
							},
							"value": [ 1.456, "2.5" ]
						},
						{
							"metric": {
								"deployment": "cf",
								"tag-name2":   "tag-value2"
							},
							"value": [ 2, "3.5" ]
						}
					]
				}
			}`), &result)
			Expect(err).ToNot(HaveOccurred())

			Expect(result.GetVector().GetSamples()).To(HaveLen(2))
			Expect(result.GetVector().GetSamples()[0].GetMetric()).To(Equal(map[string]string{
				"deployment": "cf",
				"tag-name":   "tag-value",
			}))
			Expect(result.GetVector().GetSamples()[1].GetMetric()).To(Equal(map[string]string{
				"deployment": "cf",
				"tag-name2":  "tag-value2",
			}))
			Expect(result.GetVector().GetSamples()[0].GetPoint().GetTime()).To(Equal("1.456"))
			Expect(result.GetVector().GetSamples()[1].GetPoint().GetTime()).To(Equal("2.000"))
			Expect(result.GetVector().GetSamples()[0].GetPoint().GetValue()).To(Equal(2.5))
			Expect(result.GetVector().GetSamples()[1].GetPoint().GetValue()).To(Equal(3.5))
		})

		It("handles a matrix instant query result", func() {
			marshaler := marshaler.NewPromqlMarshaler(&mockMarshaler{})

			var result logcache_v1.PromQL_InstantQueryResult
			err := marshaler.Unmarshal([]byte(`{
				"status": "success",
				"data": {
					"resultType": "matrix",
					"result": [
						{
							"metric": {
								"deployment": "cf",
								"tag-name":   "tag-value"
							},
							"values": [
								[ 1.987, "2.5" ],
								[ 2, "3.5" ]
							]
						},
						{
							"metric": {
								"deployment": "cf",
								"tag-name2":   "tag-value2"
							},
							"values": [
								[ 1, "4.5" ],
								[ 2, "6.5" ]
							]
						}
					]
				}
			}`), &result)
			Expect(err).ToNot(HaveOccurred())

			Expect(result.GetMatrix().GetSeries()).To(HaveLen(2))
			Expect(result.GetMatrix().GetSeries()[0].GetMetric()).To(Equal(map[string]string{
				"deployment": "cf",
				"tag-name":   "tag-value",
			}))
			Expect(result.GetMatrix().GetSeries()[1].GetMetric()).To(Equal(map[string]string{
				"deployment": "cf",
				"tag-name2":  "tag-value2",
			}))
			Expect(result.GetMatrix().GetSeries()[0].GetPoints()).To(HaveLen(2))
			Expect(result.GetMatrix().GetSeries()[0].GetPoints()[0].GetTime()).To(Equal("1.987"))
			Expect(result.GetMatrix().GetSeries()[0].GetPoints()[0].GetValue()).To(Equal(2.5))
			Expect(result.GetMatrix().GetSeries()[0].GetPoints()[1].GetTime()).To(Equal("2.000"))
			Expect(result.GetMatrix().GetSeries()[0].GetPoints()[1].GetValue()).To(Equal(3.5))
			Expect(result.GetMatrix().GetSeries()[1].GetPoints()).To(HaveLen(2))
			Expect(result.GetMatrix().GetSeries()[1].GetPoints()[0].GetTime()).To(Equal("1.000"))
			Expect(result.GetMatrix().GetSeries()[1].GetPoints()[0].GetValue()).To(Equal(4.5))
			Expect(result.GetMatrix().GetSeries()[1].GetPoints()[1].GetTime()).To(Equal("2.000"))
			Expect(result.GetMatrix().GetSeries()[1].GetPoints()[1].GetValue()).To(Equal(6.5))
		})

		It("handles a matrix range query result", func() {
			marshaler := marshaler.NewPromqlMarshaler(&mockMarshaler{})

			var result logcache_v1.PromQL_RangeQueryResult
			err := marshaler.Unmarshal([]byte(`{
				"status": "success",
				"data": {
					"resultType": "matrix",
					"result": [
						{
							"metric": {
								"deployment": "cf",
								"tag-name":   "tag-value"
							},
							"values": [
								[ 1.987, "2.5" ],
								[ 2, "3.5" ]
							]
						},
						{
							"metric": {
								"deployment": "cf",
								"tag-name2":   "tag-value2"
							},
							"values": [
								[ 1, "4.5" ],
								[ 2, "6.5" ]
							]
						}
					]
				}
			}`), &result)
			Expect(err).ToNot(HaveOccurred())

			Expect(result.GetMatrix().GetSeries()[0].GetPoints()).To(HaveLen(2))
			Expect(result.GetMatrix().GetSeries()[0].GetPoints()[0].GetTime()).To(Equal("1.987"))
			Expect(result.GetMatrix().GetSeries()[0].GetPoints()[0].GetValue()).To(Equal(2.5))
			Expect(result.GetMatrix().GetSeries()[0].GetPoints()[1].GetTime()).To(Equal("2.000"))
			Expect(result.GetMatrix().GetSeries()[0].GetPoints()[1].GetValue()).To(Equal(3.5))
			Expect(result.GetMatrix().GetSeries()[1].GetPoints()).To(HaveLen(2))
			Expect(result.GetMatrix().GetSeries()[1].GetPoints()[0].GetTime()).To(Equal("1.000"))
			Expect(result.GetMatrix().GetSeries()[1].GetPoints()[0].GetValue()).To(Equal(4.5))
			Expect(result.GetMatrix().GetSeries()[1].GetPoints()[1].GetTime()).To(Equal("2.000"))
			Expect(result.GetMatrix().GetSeries()[1].GetPoints()[1].GetValue()).To(Equal(6.5))
		})

		It("falls back to the fallback marshaler", func() {
			marshaler := marshaler.NewPromqlMarshaler(&mockMarshaler{})

			var result string
			err := marshaler.Unmarshal(nil, &result)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal("mock unmarshaled result"))
		})

		It("passes through errors from the fallback marshaler", func() {
			marshaler := marshaler.NewPromqlMarshaler(&mockMarshaler{
				unmarshalError: errors.New("expected error"),
			})

			var result string
			err := marshaler.Unmarshal(nil, &result)
			Expect(err).To(HaveOccurred())
		})
	})

	Context("NewDecoder()", func() {
		It("can decodes from a reader", func() {
			marshaled := strings.NewReader(`{
				"status": "success",
				"data": {
					"resultType": "scalar",
					"result": [1.123, "2.5"]
				}
			}`)
			marshaler := marshaler.NewPromqlMarshaler(&mockMarshaler{})

			var result logcache_v1.PromQL_InstantQueryResult
			err := marshaler.NewDecoder(marshaled).Decode(&result)

			Expect(err).ToNot(HaveOccurred())
			Expect(result.GetScalar().GetTime()).To(Equal("1.123"))
			Expect(result.GetScalar().GetValue()).To(Equal(2.5))
		})

		It("falls back to the fallback marshaler", func() {
			marshaler := marshaler.NewPromqlMarshaler(&mockMarshaler{})
			decoder := marshaler.NewDecoder(bytes.NewBuffer(nil))

			var result string
			err := decoder.Decode(&result)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal("mock decoded result"))
		})

		It("passes through errors from the fallback marshaler", func() {
			marshaler := marshaler.NewPromqlMarshaler(&mockMarshaler{
				decodeError: errors.New("expected error"),
			})
			decoder := marshaler.NewDecoder(bytes.NewBuffer(nil))

			var result string
			err := decoder.Decode(&result)
			Expect(err).To(HaveOccurred())
		})
	})

	Context("ContentType()", func() {
		It("returns application/json", func() {
			marshaler := marshaler.NewPromqlMarshaler(&mockMarshaler{})

			Expect(marshaler.ContentType(nil)).To(Equal("application/json"))
		})
	})
})

type mockMarshaler struct {
	marshalError   error
	unmarshalError error
	encodeError    error
	decodeError    error
}

func (m *mockMarshaler) Marshal(v interface{}) ([]byte, error) {
	return []byte("mock marshaled result"), m.marshalError
}

func (m *mockMarshaler) Unmarshal(data []byte, v interface{}) error {
	*(v.(*string)) = "mock unmarshaled result"

	return m.unmarshalError
}

func (m *mockMarshaler) NewEncoder(w io.Writer) runtime.Encoder {
	return runtime.EncoderFunc(func(interface{}) error {
		_, err := w.Write([]byte("mock encoded result"))
		if err != nil {
			return err
		}

		return m.encodeError
	})
}

func (m *mockMarshaler) NewDecoder(r io.Reader) runtime.Decoder {
	return runtime.DecoderFunc(func(v interface{}) error {
		*(v.(*string)) = "mock decoded result"

		return m.decodeError
	})
}

func (m *mockMarshaler) ContentType(interface{}) string {
	panic("not implemented")
}

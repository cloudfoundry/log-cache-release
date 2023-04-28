package tls

type TLS struct {
	CAPath   string `env:"CA_PATH,   report"`
	CertPath string `env:"CERT_PATH, report"`
	KeyPath  string `env:"KEY_PATH,  report"`
}

func (t TLS) HasAnyCredential() bool {
	return t.CAPath != "" || t.CertPath != "" || t.KeyPath != ""
}

package ipfs

// Data -
type Data struct {
	Raw          []byte
	Node         string
	ResponseTime int64
}

// Provider -
type Provider struct {
	ID      string `yaml:"id" validate:"required"`
	Address string `yaml:"addr" validate:"required"`
}

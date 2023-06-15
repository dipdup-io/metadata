package ipfs

import "errors"

// Errors
var (
	ErrInvalidURI           = errors.New("invalid URI")
	ErrEmptyIPFSGatewayList = errors.New("empty IPFS gateway list")
	ErrInvalidCID           = errors.New("invalid CID")
	ErrHTTPRequest          = errors.New("HTTP request error")
	ErrJSONDecoding         = errors.New("JSON decoding error")
	ErrNoIPFSResponse       = errors.New("can't load document from IPFS")
)

package models

// IPFSLink -
type IPFSLink struct {
	//nolint
	tableName struct{} `pg:"ipfs_link"`

	ID   int64
	Node string
	Link string `pg:",unique:ipfs"`
	Data JSONB  `pg:",type:jsonb,use_zero"`
}

// TableName -
func (IPFSLink) TableName() string {
	return "ipfs_link"
}

// IPFSLinkRepository -
type IPFSLinkRepository interface {
	IPFSLink(id int64) (IPFSLink, error)
	IPFSLinkByURL(url string) (IPFSLink, error)
	IPFSLinks(limit, offset int) ([]IPFSLink, error)
	SaveIPFSLink(link IPFSLink) error
	UpdateIPFSLink(link IPFSLink) error
}

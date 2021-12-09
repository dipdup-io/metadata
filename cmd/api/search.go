package main

import (
	"net/http"
	"strings"

	"log"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"
)

// index names
const (
	IndexToken    = "token_metadata"
	IndexContract = "contract_metadata"
)

const maxLimit = 25

type searchRequest struct {
	Index  string `query:"i"`
	Fields string `query:"f"`
	Offset int    `query:"o"`
	Limit  int    `query:"l"`
	Sort   string `query:"s"`
	Query  string `query:"q"`
}

func (req *searchRequest) validate() error {
	if req.Index == "" {
		req.Index = strings.Join([]string{IndexToken, IndexContract}, ",")
	} else {
		for _, index := range strings.Split(req.Index, ",") {
			if index != IndexToken && index != IndexContract {
				return errors.Errorf("Invalid index: %s", index)
			}
		}
	}

	if req.Limit == 0 || req.Limit > maxLimit {
		req.Limit = maxLimit
	}
	if req.Offset < 0 {
		req.Offset = 0
	}

	if req.Sort == "" {
		req.Sort = "created_at:desc"
	} else {
		parts := strings.Split(req.Sort, ":")
		if len(parts) != 2 || (parts[1] != "asc" && parts[1] != "desc") {
			return errors.Errorf("Invalid sort: %s. Should be a string of <field>:<direction> pair. direction is 'asc' or 'desc'", req.Sort)
		}
	}

	if len(req.Query) < 2 {
		return errors.Errorf("Invalid query string: %s. Should be at least 2 symbols in length", req.Query)
	}
	return nil
}

func search(c echo.Context) error {
	var req searchRequest
	if err := c.Bind(&req); err != nil {
		return err
	}

	if err := req.validate(); err != nil {
		return err
	}

	log.Println(req)

	resp, err := es.Search(
		es.Search.WithFrom(req.Offset),
		es.Search.WithIndex(req.Index),
		es.Search.WithSort(req.Sort),
		es.Search.WithSize(req.Limit),
		es.Search.WithQuery(req.Query),
	)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.IsError() {
		return errors.New(resp.String())
	}

	return c.Stream(http.StatusOK, "application/json", resp.Body)
}

package treediagram

import (
	"github.com/jukeizu/contract"
	"github.com/jukeizu/management/pkg/management"
)

func FormatError(err error) (*contract.Response, error) {

	switch err.(type) {
	case management.ValidationError:
		return contract.StringResponse(err.Error()), nil
	}

	return nil, err
}

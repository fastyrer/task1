package storage

import "errors"

var (
	ErrContactAlreadyExists = errors.New("contact with this phone already exists")
	ErrContactNotFound      = errors.New("contact not found")
)

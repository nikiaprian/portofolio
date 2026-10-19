package services

import "errors"

var (
	ErrSiteLayoutInUse     = errors.New("site layout is in use")
	ErrSiteLayoutNameTaken = errors.New("site layout name already taken")
	ErrSiteLayoutNotFound  = errors.New("site layout not found")
)

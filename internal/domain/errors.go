package domain

import "errors"

var (
	ErrNotFound      = errors.New("сценарий не найден")
	ErrInvalidYAML   = errors.New("некорректный YAML")
	ErrMissingSerial = errors.New("укажите serial")
	ErrMissingID     = errors.New("укажите id сценария")
)

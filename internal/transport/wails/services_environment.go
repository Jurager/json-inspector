package wails

import "json-inspector/internal/infra/keychain"

// EnvironmentsService is the environments surface. Secrets are the only part of it that still
// lives on this side: they sit in the OS keychain until the one-time import moves them into the
// database, and this service is that import's read path as well as the UI's.
type EnvironmentsService struct{}

func NewEnvironmentsService() *EnvironmentsService {
	return &EnvironmentsService{}
}

func (s *EnvironmentsService) SecretSet(envID, name, value string) error {
	return keychain.Set(envID, name, value)
}

func (s *EnvironmentsService) SecretGet(envID, name string) (string, error) {
	return keychain.Get(envID, name)
}

func (s *EnvironmentsService) SecretDelete(envID, name string) error {
	return keychain.Delete(envID, name)
}

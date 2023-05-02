package waiter

import "github.com/google/uuid"

type IDGenerator interface {
	// ID should return a random ID for uniquely
	// identifying a request. Will be called multiple
	// times if there are collisions
	ID() (string, error)
}

type UUIDGenerator struct{}

var _ IDGenerator = (*UUIDGenerator)(nil)

func (g *UUIDGenerator) ID() (string, error) {
	uid, err := uuid.NewRandom()
	if err != nil {
		return "", err
	}

	return uid.String(), nil
}

type IDGeneratorFuncs struct {
	IDFunc func() (string, error)
}

func (i IDGeneratorFuncs) ID() (string, error) {
	return i.IDFunc()
}

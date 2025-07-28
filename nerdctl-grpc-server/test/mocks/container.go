package mocks

import (
	"context"
)

// MockContainerManager is a mock implementation of container manager
type MockContainerManager struct {
	containers map[string]*MockContainer
}

// MockContainer represents a mock container
type MockContainer struct {
	ID     string
	Name   string
	Status string
	Image  string
}

// NewMockContainerManager creates a new mock container manager
func NewMockContainerManager() *MockContainerManager {
	return &MockContainerManager{
		containers: make(map[string]*MockContainer),
	}
}

// CreateContainer creates a mock container
func (m *MockContainerManager) CreateContainer(ctx context.Context, name, image string) (*MockContainer, error) {
	container := &MockContainer{
		ID:     "mock-id-" + name,
		Name:   name,
		Status: "created",
		Image:  image,
	}
	m.containers[container.ID] = container
	return container, nil
}

// GetContainer gets a mock container by ID
func (m *MockContainerManager) GetContainer(ctx context.Context, id string) (*MockContainer, error) {
	if container, exists := m.containers[id]; exists {
		return container, nil
	}
	return nil, ErrContainerNotFound
}

// ListContainers lists all mock containers
func (m *MockContainerManager) ListContainers(ctx context.Context) ([]*MockContainer, error) {
	var containers []*MockContainer
	for _, container := range m.containers {
		containers = append(containers, container)
	}
	return containers, nil
}

// DeleteContainer deletes a mock container
func (m *MockContainerManager) DeleteContainer(ctx context.Context, id string) error {
	if _, exists := m.containers[id]; exists {
		delete(m.containers, id)
		return nil
	}
	return ErrContainerNotFound
}
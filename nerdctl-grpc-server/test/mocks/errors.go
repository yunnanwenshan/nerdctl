package mocks

import "errors"

var (
	// ErrContainerNotFound is returned when a container is not found
	ErrContainerNotFound = errors.New("container not found")
	
	// ErrImageNotFound is returned when an image is not found
	ErrImageNotFound = errors.New("image not found")
	
	// ErrNetworkNotFound is returned when a network is not found
	ErrNetworkNotFound = errors.New("network not found")
)
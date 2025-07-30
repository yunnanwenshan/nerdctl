package compatibility

import (
	"fmt"
)

// Version represents a nerdctl version
type Version struct {
	Major int
	Minor int
	Patch int
}

// String returns the version as a string
func (v Version) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// ParseVersion parses a version string into a Version struct
func ParseVersion(versionStr string) (*Version, error) {
	var major, minor, patch int
	_, err := fmt.Sscanf(versionStr, "%d.%d.%d", &major, &minor, &patch)
	if err != nil {
		return nil, fmt.Errorf("failed to parse version %s: %w", versionStr, err)
	}
	
	return &Version{
		Major: major,
		Minor: minor,
		Patch: patch,
	}, nil
}

// IsCompatible checks if the given version is compatible
func (v Version) IsCompatible(other Version) bool {
	// Simple compatibility check - same major version
	return v.Major == other.Major
}

// GetSupportedVersions returns list of supported nerdctl versions
func GetSupportedVersions() []Version {
	return []Version{
		{Major: 1, Minor: 0, Patch: 0},
		{Major: 1, Minor: 1, Patch: 0},
		{Major: 1, Minor: 2, Patch: 0},
	}
}
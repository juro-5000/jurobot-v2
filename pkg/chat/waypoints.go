package chat

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var waypointRegex = regexp.MustCompile(`xaero-waypoint:[^:]+:[^:]+:-?\d+:-?\d+:-?\d+:\d+:(true|false):\d+:[^:\s]+`)

// Waypoint represents a Xaero's Waypoint.
type Waypoint struct {
	Name      string
	Symbol    string
	X         int
	Y         int
	Z         int
	Color     int
	Visible   bool
	Rotation  int
	Dimension string
}

// ParseWaypoint parses a Xaero's waypoint string.
// Format: xaero-waypoint:<name>:<symbol>:<x>:<y>:<z>:<color>:<visible>:<rotation>:<internal-dimension-id>
func ParseWaypoint(message string) (*Waypoint, error) {
	if !strings.HasPrefix(message, "xaero-waypoint:") {
		return nil, fmt.Errorf("not a xaero-waypoint")
	}

	parts := strings.Split(message, ":")
	if len(parts) < 10 {
		return nil, fmt.Errorf("invalid xaero-waypoint format: too few parts")
	}

	// Work from the end because the name might contain colons
	n := len(parts)
	dimension := parts[n-1]
	rotationStr := parts[n-2]
	visibleStr := parts[n-3]
	colorStr := parts[n-4]
	zStr := parts[n-5]
	yStr := parts[n-6]
	xStr := parts[n-7]
	symbol := parts[n-8]

	// The name is everything between parts[0] and parts[n-8]
	name := strings.Join(parts[1:n-8], ":")

	x, err := strconv.Atoi(xStr)
	if err != nil {
		return nil, fmt.Errorf("invalid X coordinate: %v", err)
	}
	y, err := strconv.Atoi(yStr)
	if err != nil {
		return nil, fmt.Errorf("invalid Y coordinate: %v", err)
	}
	z, err := strconv.Atoi(zStr)
	if err != nil {
		return nil, fmt.Errorf("invalid Z coordinate: %v", err)
	}
	color, err := strconv.Atoi(colorStr)
	if err != nil {
		return nil, fmt.Errorf("invalid color: %v", err)
	}
	visible, err := strconv.ParseBool(visibleStr)
	if err != nil {
		return nil, fmt.Errorf("invalid visible flag: %v", err)
	}
	rotation, err := strconv.Atoi(rotationStr)
	if err != nil {
		return nil, fmt.Errorf("invalid rotation: %v", err)
	}

	return &Waypoint{
		Name:      name,
		Symbol:    symbol,
		X:         x,
		Y:         y,
		Z:         z,
		Color:     color,
		Visible:   visible,
		Rotation:  rotation,
		Dimension: dimension,
	}, nil
}

// FindWaypoint searches for a xaero-waypoint in a string and returns the parsed waypoint.
func FindWaypoint(message string) (*Waypoint, error) {
	match := waypointRegex.FindString(message)
	if match == "" {
		return nil, fmt.Errorf("no xaero-waypoint found")
	}
	return ParseWaypoint(match)
}

// ReplaceWaypoints replaces all xaero-waypoint strings in a message with their pretty representation.
func ReplaceWaypoints(message string) string {
	return waypointRegex.ReplaceAllStringFunc(message, func(match string) string {
		if wp, err := ParseWaypoint(match); err == nil {
			return wp.String()
		}
		return match
	})
}

func (w *Waypoint) String() string {
	dim := w.Dimension
	dim = strings.TrimPrefix(dim, "Internal-")
	dim = strings.ReplaceAll(dim, "_", " ")
	return fmt.Sprintf("Waypoint[%s, %d %d %d, %s]", 
		w.Name, w.X, w.Y, w.Z, dim)
}

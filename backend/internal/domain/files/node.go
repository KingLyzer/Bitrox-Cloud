package files

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

type NodeType string

const (
	NodeTypeFile   NodeType = "file"
	NodeTypeFolder NodeType = "folder"
)

type Node struct {
	ID               uuid.UUID
	OwnerUserID      uuid.UUID
	ParentID         *uuid.UUID
	Type             NodeType
	Name             string
	SizeBytes        int64
	MIMEType         *string
	ContentHash      *string
	StorageKey       *string
	CurrentVersionID *uuid.UUID
	CurrentVersionNo *int
	DeletedAt        *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type FileVersion struct {
	ID              uuid.UUID
	NodeID          uuid.UUID
	VersionNo       int
	StorageKey      string
	SizeBytes       int64
	MIMEType        string
	ContentHash     string
	CreatedByUserID uuid.UUID
	CreatedAt       time.Time
}

func IsValidNodeType(nodeType NodeType) bool {
	return nodeType == NodeTypeFile || nodeType == NodeTypeFolder
}

func NormalizeNodeName(name string) string {
	return strings.TrimSpace(name)
}

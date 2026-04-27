package quota

import "github.com/google/uuid"

type Usage struct {
	UserID          uuid.UUID
	UsedBytes       int64
	LimitBytes      int64
	RemainingBytes  int64
	IsLimitExceeded bool
}

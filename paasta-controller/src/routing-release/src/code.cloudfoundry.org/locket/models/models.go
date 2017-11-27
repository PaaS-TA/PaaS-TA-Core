package models

import "errors"

//go:generate bash ../scripts/generate_protos.sh
//go:generate counterfeiter . LocketClient

const PresenceType = "presence"
const LockType = "lock"

var ErrLockCollision = errors.New("lock-collision")
var ErrInvalidTTL = errors.New("invalid-ttl")
var ErrInvalidOwner = errors.New("invalid-owner")
var ErrResourceNotFound = errors.New("resource-not-found")

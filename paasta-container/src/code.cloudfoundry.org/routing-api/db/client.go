package db

import "github.com/jinzhu/gorm"

//go:generate counterfeiter -o fakes/fake_client.go . Client
type Client interface {
	Close() error
	Create(value interface{}) *gorm.DB
	Delete(value interface{}, where ...interface{}) *gorm.DB
	Save(value interface{}) *gorm.DB
	Update(attrs ...interface{}) *gorm.DB
	Where(query interface{}, args ...interface{}) *gorm.DB
	First(out interface{}, where ...interface{}) *gorm.DB
	Find(out interface{}, where ...interface{}) *gorm.DB
}

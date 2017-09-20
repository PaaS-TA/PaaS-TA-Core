package db_test

import (
	"errors"

	"code.cloudfoundry.org/routing-api/config"
	"code.cloudfoundry.org/routing-api/db"
	"code.cloudfoundry.org/routing-api/db/fakes"
	"code.cloudfoundry.org/routing-api/models"
	"github.com/jinzhu/gorm"
	"github.com/nu7hatch/gouuid"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("SqlDB", func() {
	var (
		sqlDB *db.SqlDB
		err   error
	)
	BeforeEach(func() {
		sqlCfg = &config.SqlDB{
			Username: "root",
			Password: "password",
			Schema:   sqlDBName,
			Host:     "localhost",
			Port:     3306,
			Type:     "mysql",
		}
		dbSQL, err := db.NewSqlDB(sqlCfg)
		Expect(err).ToNot(HaveOccurred())
		sqlDB = dbSQL.(*db.SqlDB)
	})

	Describe("Connection", func() {
		var sqlDB db.DB
		JustBeforeEach(func() {
			sqlDB, err = db.NewSqlDB(sqlCfg)
		})

		It("returns a sql db client", func() {
			Expect(err).ToNot(HaveOccurred())
			Expect(sqlDB).ToNot(BeNil())
		})

		Context("when config is nil", func() {
			BeforeEach(func() {
				sqlCfg = nil
			})

			It("returns an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(sqlDB).To(BeNil())
			})
		})

		Context("when authentication fails", func() {
			BeforeEach(func() {
				sqlCfg.Password = "wrong_password"
			})

			It("returns an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(sqlDB).To(BeNil())
			})
		})

		Context("when connecting to SQL DB fails", func() {
			BeforeEach(func() {
				sqlCfg.Port = 1234
			})

			It("returns an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(sqlDB).To(BeNil())

			})
		})
	})

	Describe("ReadRouterGroups", func() {
		var (
			routerGroups models.RouterGroups
			err          error
			rg           models.RouterGroupDB
		)

		JustBeforeEach(func() {
			routerGroups, err = sqlDB.ReadRouterGroups()
		})

		Context("when there are router groups", func() {
			BeforeEach(func() {
				rg = models.RouterGroupDB{
					Guid:            newUuid(),
					Name:            "rg-1",
					Type:            "tcp",
					ReservablePorts: "120",
				}
				Expect(sqlDB.Client.Create(&rg).Error).ToNot(HaveOccurred())
			})

			AfterEach(func() {
				Expect(sqlDB.Client.Delete(&rg).Error).ToNot(HaveOccurred())
			})

			It("returns list of router groups", func() {
				Expect(err).ToNot(HaveOccurred())
				Expect(routerGroups).ToNot(BeNil())
				Expect(len(routerGroups)).To(BeNumerically(">", 0))
				Expect(routerGroups).Should(ContainElement(rg.ToRouterGroup()))
			})
		})

		Context("when there are no router groups", func() {
			BeforeEach(func() {
				sqlDB.Client.Where("1 = 1").Delete(&models.RouterGroupDB{})
			})

			It("returns an empty slice", func() {
				Expect(err).ToNot(HaveOccurred())
				Expect(routerGroups).ToNot(BeNil())
				Expect(routerGroups).To(HaveLen(0))
			})
		})

		Context("when there is a connection error", func() {
			BeforeEach(func() {
				fakeClient := &fakes.FakeClient{}
				fakeClient.FindReturns(&gorm.DB{Error: errors.New("connection refused")})
				sqlDB.Client = fakeClient
			})

			It("returns an error", func() {
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("ReadRouterGroup", func() {
		var (
			routerGroup   models.RouterGroup
			err           error
			rg            models.RouterGroupDB
			routerGroupId string
		)

		JustBeforeEach(func() {
			routerGroup, err = sqlDB.ReadRouterGroup(routerGroupId)
		})

		Context("when router group exists", func() {
			BeforeEach(func() {
				routerGroupId = newUuid()
				rg = models.RouterGroupDB{
					Guid:            routerGroupId,
					Name:            "rg-1",
					Type:            "tcp",
					ReservablePorts: "120",
				}
				Expect(sqlDB.Client.Create(&rg).Error).ToNot(HaveOccurred())
			})

			AfterEach(func() {
				Expect(sqlDB.Client.Delete(&rg).Error).ToNot(HaveOccurred())
			})

			It("returns the router group", func() {
				Expect(err).ToNot(HaveOccurred())
				Expect(routerGroup.Guid).To(Equal(rg.Guid))
				Expect(routerGroup.Name).To(Equal(rg.Name))
				Expect(string(routerGroup.ReservablePorts)).To(Equal(rg.ReservablePorts))
				Expect(string(routerGroup.Type)).To(Equal(rg.Type))
			})
		})

		Context("when router group doesn't exist", func() {
			BeforeEach(func() {
				routerGroupId = newUuid()
			})

			It("returns an empty struct", func() {
				Expect(err).ToNot(HaveOccurred())
				Expect(routerGroup).To(Equal(models.RouterGroup{}))
			})
		})
	})

	Describe("SaveRouterGroup", func() {
		var (
			routerGroup   models.RouterGroup
			err           error
			routerGroupId string
		)
		BeforeEach(func() {
			routerGroupId = newUuid()
			routerGroup = models.RouterGroup{
				Guid:            routerGroupId,
				Name:            "router-group-1",
				Type:            "tcp",
				ReservablePorts: "65000-65002",
			}
		})

		JustBeforeEach(func() {
			err = sqlDB.SaveRouterGroup(routerGroup)
		})

		Context("when router group exists", func() {
			BeforeEach(func() {
				sqlDB.Client.Create(&models.RouterGroupDB{
					Guid:            routerGroupId,
					Name:            "rg-1",
					Type:            "tcp",
					ReservablePorts: "120",
				})
			})

			AfterEach(func() {
				sqlDB.Client.Delete(&models.RouterGroupDB{
					Guid: routerGroupId,
				})
			})

			It("updates the existing router group", func() {
				Expect(err).ToNot(HaveOccurred())
				rg, err := sqlDB.ReadRouterGroup(routerGroup.Guid)
				Expect(err).ToNot(HaveOccurred())

				Expect(rg.Guid).To(Equal(routerGroup.Guid))
				Expect(rg.Name).To(Equal(routerGroup.Name))
				Expect(rg.ReservablePorts).To(Equal(routerGroup.ReservablePorts))
				Expect(rg.Type).To(Equal(routerGroup.Type))
			})
		})

		Context("when router group doesn't exist", func() {
			It("creates the router group", func() {
				Expect(err).ToNot(HaveOccurred())
				rg, err := sqlDB.ReadRouterGroup(routerGroup.Guid)
				Expect(err).ToNot(HaveOccurred())
				Expect(rg.Guid).To(Equal(routerGroup.Guid))
				Expect(rg.Name).To(Equal(routerGroup.Name))
				Expect(rg.ReservablePorts).To(Equal(routerGroup.ReservablePorts))
				Expect(rg.Type).To(Equal(routerGroup.Type))
			})
		})
	})

	Describe("Methods not implemented", func() {
		It("returns an error", func() {
			err := sqlDB.SaveRoute(models.Route{})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("function not implemented:"))
			Expect(err.Error()).To(ContainSubstring("SaveRoute"))
		})
	})
})

func newUuid() string {
	u, err := uuid.NewV4()
	Expect(err).ToNot(HaveOccurred())
	return u.String()
}

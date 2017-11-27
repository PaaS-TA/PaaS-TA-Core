package migration_test

import (
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/routing-api/cmd/routing-api/testrunner"
	"code.cloudfoundry.org/routing-api/config"
	"code.cloudfoundry.org/routing-api/db"
	"code.cloudfoundry.org/routing-api/migration"
	"code.cloudfoundry.org/routing-api/models"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("V2UpdateRgMigration", func() {
	var (
		sqlDB          *db.SqlDB
		mysqlAllocator testrunner.DbAllocator
		etcdConfig     *config.Etcd
		done           chan struct{}
		logger         lager.Logger
	)

	BeforeEach(func() {
		mysqlAllocator = testrunner.NewMySQLAllocator()
		mysqlSchema, err := mysqlAllocator.Create()
		Expect(err).NotTo(HaveOccurred())

		logger = lager.NewLogger("test-logger")

		sqlCfg := &config.SqlDB{
			Username: "root",
			Password: "password",
			Schema:   mysqlSchema,
			Host:     "localhost",
			Port:     3306,
			Type:     "mysql",
		}

		sqlDB, err = db.NewSqlDB(sqlCfg)
		Expect(err).ToNot(HaveOccurred())

		v0Migration := migration.NewV0InitMigration()
		err = v0Migration.Run(sqlDB)
		Expect(err).ToNot(HaveOccurred())

		etcdConfig = &config.Etcd{}
		done = make(chan struct{})

		v1Migration := migration.NewV1EtcdMigration(etcdConfig, done, logger)
		err = v1Migration.Run(sqlDB)
		Expect(err).ToNot(HaveOccurred())

	})
	AfterEach(func() {
		err := mysqlAllocator.Delete()
		Expect(err).ToNot(HaveOccurred())
	})

	Describe("Version", func() {
		It("returns 2 for the version", func() {
			v2Migration := migration.NewV2UpdateRgMigration()
			Expect(v2Migration.Version()).To(Equal(2))
		})
	})

	Describe("Run", func() {
		Context("when no records exist", func() {
			BeforeEach(func() {
				v2Migration := migration.NewV2UpdateRgMigration()
				err := v2Migration.Run(sqlDB)
				Expect(err).ToNot(HaveOccurred())

				rg, err := sqlDB.ReadRouterGroups()
				Expect(err).NotTo(HaveOccurred())
				Expect(rg).To(HaveLen(0))
			})

			It("does not allow duplicate router group names", func() {
				rg1 := models.RouterGroupDB{
					Model:           models.Model{Guid: "guid-1"},
					Name:            "rg-1",
					Type:            "tcp",
					ReservablePorts: "120",
				}

				rg2 := models.RouterGroupDB{
					Model:           models.Model{Guid: "guid-2"},
					Name:            "rg-1",
					Type:            "tcp",
					ReservablePorts: "120",
				}

				_, err := sqlDB.Client.Create(&rg1)
				Expect(err).NotTo(HaveOccurred())

				_, err = sqlDB.Client.Create(&rg2)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Duplicate"))
			})
		})

		Context("when there are existing records", func() {
			BeforeEach(func() {
				rg1 := models.RouterGroupDB{
					Model:           models.Model{Guid: "guid-1"},
					Name:            "rg-1",
					Type:            "tcp",
					ReservablePorts: "120",
				}
				_, err := sqlDB.Client.Create(&rg1)
				Expect(err).NotTo(HaveOccurred())
			})

			Context("when the existing router groups have unique names", func() {
				BeforeEach(func() {
					rg2 := models.RouterGroupDB{
						Model:           models.Model{Guid: "guid-2"},
						Name:            "rg-2",
						Type:            "tcp",
						ReservablePorts: "120",
					}
					_, err := sqlDB.Client.Create(&rg2)
					Expect(err).NotTo(HaveOccurred())
				})
				It("should successfully migrate", func() {
					v2Migration := migration.NewV2UpdateRgMigration()
					err := v2Migration.Run(sqlDB)
					Expect(err).NotTo(HaveOccurred())
					rg, err := sqlDB.ReadRouterGroups()
					Expect(err).NotTo(HaveOccurred())
					Expect(rg).To(HaveLen(2))
				})
			})

			Context("when the existing router groups do not have unique names", func() {
				BeforeEach(func() {
					rg2 := models.RouterGroupDB{
						Model:           models.Model{Guid: "guid-2"},
						Name:            "rg-1",
						Type:            "tcp",
						ReservablePorts: "120",
					}
					_, err := sqlDB.Client.Create(&rg2)
					Expect(err).NotTo(HaveOccurred())
				})

				It("should fail migration", func() {
					v2Migration := migration.NewV2UpdateRgMigration()
					err := v2Migration.Run(sqlDB)
					Expect(err).To(HaveOccurred())
				})
			})
		})
	})
})

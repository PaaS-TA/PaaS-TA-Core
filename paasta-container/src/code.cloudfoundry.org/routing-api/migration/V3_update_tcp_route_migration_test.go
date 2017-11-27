package migration_test

import (
	"time"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/routing-api/cmd/routing-api/testrunner"
	"code.cloudfoundry.org/routing-api/config"
	"code.cloudfoundry.org/routing-api/db"
	"code.cloudfoundry.org/routing-api/migration"
	"code.cloudfoundry.org/routing-api/models"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("V3UpdateTcpRouteMigration", func() {
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

		tcpRoute1Entity := models.TcpMappingEntity{
			RouterGroupGuid: "rg-guid-1",
			HostPort:        2000,
			HostIP:          "1.2.3.4",
			ExternalPort:    3000,
		}
		tcpRoute1 := models.TcpRouteMapping{
			Model:            models.Model{Guid: "guid-1"},
			ExpiresAt:        time.Now().Add(1 * time.Hour),
			TcpMappingEntity: tcpRoute1Entity,
		}
		foo, err := sqlDB.Client.Create(&tcpRoute1)
		Expect(int(foo)).To(Equal(1))
		Expect(err).NotTo(HaveOccurred())
		tcpRoutes, err := sqlDB.ReadTcpRouteMappings()
		Expect(err).NotTo(HaveOccurred())
		Expect(tcpRoutes).To(HaveLen(1))

		err = sqlDB.Client.Model(&models.TcpRouteMapping{}).DropColumn("isolation_segment")
		Expect(err).ToNot(HaveOccurred())

		rows, err := sqlDB.Client.Rows("tcp_routes")
		Expect(err).ToNot(HaveOccurred())
		columnList, err := rows.Columns()
		Expect(err).ToNot(HaveOccurred())
		Expect(columnList).ToNot(ContainElement("isolation_segment"))

		etcdConfig = &config.Etcd{}
		done = make(chan struct{})

		v1Migration := migration.NewV1EtcdMigration(etcdConfig, done, logger)
		err = v1Migration.Run(sqlDB)
		Expect(err).ToNot(HaveOccurred())

		v2Migration := migration.NewV2UpdateRgMigration()
		err = v2Migration.Run(sqlDB)
		Expect(err).ToNot(HaveOccurred())
	})
	AfterEach(func() {
		err := mysqlAllocator.Delete()
		Expect(err).ToNot(HaveOccurred())
	})

	Describe("Version", func() {
		It("returns 3 for the version", func() {
			v3Migration := migration.NewV3UpdateTcpRouteMigration()
			Expect(v3Migration.Version()).To(Equal(3))
		})
	})

	Describe("Run", func() {
		var v3Migration migration.Migration
		BeforeEach(func() {
			v3Migration = migration.NewV3UpdateTcpRouteMigration()
		})

		It("should update table definition to include isolation_segments column", func() {
			err := v3Migration.Run(sqlDB)
			Expect(err).ToNot(HaveOccurred())
			Expect(sqlDB.Client.HasTable(&models.TcpRouteMapping{})).To(BeTrue())

			rows, err := sqlDB.Client.Rows("tcp_routes")
			Expect(err).ToNot(HaveOccurred())
			columnList, err := rows.Columns()
			Expect(err).ToNot(HaveOccurred())
			Expect(columnList).Should(ContainElement("isolation_segment"))
		})

		It("should update existing records with empty isolation segments", func() {
			tcpRoutes, err := sqlDB.ReadTcpRouteMappings()
			Expect(err).NotTo(HaveOccurred())
			Expect(tcpRoutes).To(HaveLen(1))
			err = v3Migration.Run(sqlDB)
			Expect(err).ToNot(HaveOccurred())
			Expect(sqlDB.Client.HasTable(&models.TcpRouteMapping{})).To(BeTrue())
		})
	})
})

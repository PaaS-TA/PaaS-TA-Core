package helpers_test

import (
	"github.com/cloudfoundry/postgres-release/src/acceptance-tests/testing/helpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Postgres load sets", func() {
	Context("Generate valid tables lists", func() {
		It("Correctly creates a table list", func() {
			tables := helpers.GetSampleLoad(helpers.Test2Load)
			expected := []helpers.PGLoadTable{
				helpers.PGLoadTable{
					Name: "pgats_table_0",
					ColumnNames: []string{
						"column0",
						"column1",
						"column2",
						"column3",
					},
					ColumnTypes: []string{
						helpers.RowSamples[0][0].(string),
						helpers.RowSamples[1][0].(string),
						helpers.RowSamples[2][0].(string),
						helpers.RowSamples[0][0].(string),
					},
					SampleRow: []interface{}{
						helpers.RowSamples[0][1],
						helpers.RowSamples[1][1],
						helpers.RowSamples[2][1],
						helpers.RowSamples[0][1],
					},
					NumRows: 5,
				},
				helpers.PGLoadTable{
					Name: "pgats_table_1",
					ColumnNames: []string{
						"column0",
						"column1",
						"column2",
						"column3",
					},
					ColumnTypes: []string{
						helpers.RowSamples[0][0].(string),
						helpers.RowSamples[1][0].(string),
						helpers.RowSamples[2][0].(string),
						helpers.RowSamples[0][0].(string),
					},
					SampleRow: []interface{}{
						helpers.RowSamples[0][1],
						helpers.RowSamples[1][1],
						helpers.RowSamples[2][1],
						helpers.RowSamples[0][1],
					},
					NumRows: 5,
				},
			}
			Expect(tables).To(Equal(expected))
		})
	})
	Context("With a good table in input", func() {
		var table helpers.PGLoadTable

		BeforeEach(func() {
			table = helpers.PGLoadTable{
				Name: "test1",
				ColumnNames: []string{
					"column1",
					"column2",
					"column3",
				},
				ColumnTypes: []string{
					"character varying not null",
					"integer",
					"notmanaged",
				},
				SampleRow: []interface{}{
					"sample",
					5,
					false,
				},
				NumRows: 2,
			}
		})

		It("Corretly prepare CREATE", func() {
			expected := `CREATE TABLE test1 (column1 character varying not null,
column2 integer,
column3 notmanaged);`
			Expect(table.PrepareCreate()).To(Equal(expected))
		})
		It("Corretly prepare statement", func() {
			expected := `COPY "test1" ("column1", "column2", "column3") FROM STDIN`
			Expect(table.PrepareStatement()).To(Equal(expected))
		})
		It("Corretly prepare row", func() {
			idx := 2
			expected := []interface{}{"sample2", 2, false}
			Expect(table.PrepareRow(idx)).To(Equal(expected))
		})
	})
	Context("With a table without rows", func() {
		var table helpers.PGLoadTable

		BeforeEach(func() {
			table.Name = "test1"
		})

		It("Corretly prepare CREATE", func() {
			expected := `CREATE TABLE test1 ();`
			Expect(table.PrepareCreate()).To(Equal(expected))
		})
		It("Corretly prepare statement", func() {
			var expected string
			Expect(table.PrepareStatement()).To(Equal(expected))
		})
		It("Corretly prepare row", func() {
			idx := 0
			var expected []interface{}
			Expect(table.PrepareRow(idx)).To(Equal(expected))
		})
	})
	Context("With an inconsistent table in input", func() {

		It("Uses default column type in CREATE if type missing", func() {
			var table = helpers.PGLoadTable{
				Name: "test1",
				ColumnNames: []string{
					"column1",
					"column2",
				},
				ColumnTypes: []string{
					"character varying not null",
				},
			}
			expected := `CREATE TABLE test1 (column1 character varying not null,
column2 character varying);`
			Expect(table.PrepareCreate()).To(Equal(expected))
		})
		It("Returns empty prepared statement if table name is missing", func() {
			var table = helpers.PGLoadTable{
				ColumnNames: []string{
					"column1",
				},
				ColumnTypes: []string{
					"character varying not null",
				},
			}
			var expected string
			Expect(table.PrepareStatement()).To(Equal(expected))
		})
		It("Returns empty row if no colums", func() {
			var table = helpers.PGLoadTable{
				ColumnTypes: []string{
					"character varying not null",
				},
			}
			var expected []interface{}
			Expect(table.PrepareRow(0)).To(Equal(expected))
		})
		It("Ignores extra sample values preparing row", func() {
			var table = helpers.PGLoadTable{
				ColumnNames: []string{
					"column1",
				},
				SampleRow: []interface{}{
					"sample",
					5,
				},
				NumRows: 2,
			}
			idx := 2
			expected := []interface{}{"sample2"}
			Expect(table.PrepareRow(idx)).To(Equal(expected))
		})
	})
})

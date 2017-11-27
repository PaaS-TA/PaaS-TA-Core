package helpers

import (
	"fmt"
	"strings"
	"time"

	pq "github.com/lib/pq"
)

type PGLoadTable struct {
	Name        string
	ColumnNames []string
	ColumnTypes []string
	SampleRow   []interface{}
	NumRows     int
}

var RowSamples = [][]interface{}{
	[]interface{}{"character varying not null", "short_string"},
	[]interface{}{"character varying not null", "long_string_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"},
	[]interface{}{"integer", 0},
}

type LoadType struct {
	NumTables  int
	NumColumns int
	NumRows    int
}

var Test1Load = LoadType{NumTables: 1, NumColumns: 1, NumRows: 1}
var Test2Load = LoadType{NumTables: 2, NumColumns: 4, NumRows: 5}
var SmallLoad = LoadType{NumTables: 2, NumColumns: 10, NumRows: 50}
var MediumLoad = LoadType{NumTables: 10, NumColumns: 10, NumRows: 100}
var LargeLoad = LoadType{NumTables: 100, NumColumns: 10, NumRows: 20000}

func GetSampleLoad(loadType LoadType) []PGLoadTable {
	if loadType.NumTables <= 0 {
		return nil
	}
	var result = make([]PGLoadTable, loadType.NumTables)
	rowTypesNum := len(RowSamples)
	for i := 0; i < loadType.NumTables; i++ {
		var table = PGLoadTable{
			Name:        fmt.Sprintf("pgats_table_%d", i),
			ColumnNames: make([]string, loadType.NumColumns),
			ColumnTypes: make([]string, loadType.NumColumns),
			SampleRow:   make([]interface{}, loadType.NumColumns),
			NumRows:     loadType.NumRows,
		}
		for j := 0; j < loadType.NumColumns; j++ {
			idx := j % rowTypesNum
			table.ColumnNames[j] = fmt.Sprintf("column%d", j)
			table.ColumnTypes[j] = RowSamples[idx][0].(string)
			table.SampleRow[j] = RowSamples[idx][1]
		}
		result[i] = table
	}
	return result
}

func (table PGLoadTable) PrepareCreate() string {
	columns := make([]string, len(table.ColumnNames))
	for idx, name := range table.ColumnNames {
		var dataType string
		if idx > len(table.ColumnTypes)-1 {
			dataType = "character varying"
		} else {
			dataType = table.ColumnTypes[idx]
		}
		columns[idx] = fmt.Sprintf("%s %s", name, dataType)
	}
	return fmt.Sprintf("CREATE TABLE %s (%s);", table.Name, strings.Join(columns, ",\n"))
}

func (table PGLoadTable) PrepareStatement() string {
	var result string
	if len(table.ColumnNames) > 0 && table.Name != "" {
		result = pq.CopyIn(table.Name, table.ColumnNames...)
	}
	return result
}
func (table PGLoadTable) PrepareRow(rowIdx int) []interface{} {
	var result []interface{}
	if len(table.ColumnNames) > 0 {
		for idx, value := range table.SampleRow {
			if idx > len(table.ColumnNames)-1 {
				break
			}
			var out interface{}
			switch value.(type) {
			case string:
				out = fmt.Sprintf("%s%d", value, rowIdx)
			case int32, int64, int:
				out = rowIdx
			case time.Time:
				out = time.Now()
			default:
				out = value
			}
			result = append(result, out)
		}
	}
	return result
}

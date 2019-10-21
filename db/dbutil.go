package db

// GetEverythingFromTable dumps all rows from the given table into raw strings.
// Each element in the returned value is a row, and each column within that row
// is a element of a list.
func GetEverythingFromTable(db *DB, table string) ([][]string, error) {
	selectRows, err := db.Query(`SELECT * FROM ` + table)
	if err != nil {
		return [][]string{}, err
	}
	cols, err := selectRows.Columns()
	if err != nil {
		return [][]string{}, err
	}

	rawResult := make([][]byte, len(cols))
	result := [][]string{}

	dest := make([]interface{}, len(cols))

	for i := range rawResult {
		dest[i] = &rawResult[i]
	}

	for rowIndex := 0; selectRows.Next(); rowIndex++ {
		result = append(result, make([]string, len(cols)))
		if err = selectRows.Scan(dest...); err != nil {
			return [][]string{}, err
		}
		for i, raw := range rawResult {
			if raw == nil {
				result[rowIndex][i] = "NULL"
			} else {
				result[rowIndex][i] = string(raw)
			}
		}
	}

	return result, nil
}

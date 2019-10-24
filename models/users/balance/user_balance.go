package balance

import "gitlab.com/arcanecrypto/teslacoil/db"

// CalculateBalance calculates the balance for a given user
func CalculateBalance(db *db.DB, userID int) (int, error) {
	type balanceResult struct {
		Balance int `db:"balance"`
	}

	var result balanceResult

	query := "SELECT sum(amount_milli_sat) as balance from transactions WHERE settled_at IS NOT NULL OR confirmed_at IS NOT NULL AND user_id=$1;"

	if err := db.Get(&result, query, userID); err != nil {
		return -1, err
	}

	return result.Balance, nil
}

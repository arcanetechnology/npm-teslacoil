package main

import (
	"github.com/jinzhu/gorm"
	"gitlab.com/arcanecrypto/lpp/internal/transactions"
	"gitlab.com/arcanecrypto/lpp/internal/users"
)

// FillWithDummyData creates three entries in each table
func FillWithDummyData(d *gorm.DB) error {
	user, err := users.Create(d, users.UserNew{
		Email:    "test@example.com",
		Password: "Somehing",
	})
	if err != nil {
		return err
	}

	user.Balance = 30000000
	d.Save(user)

	_, err = transactions.Create(d, transactions.NewTransaction{
		UserID:      user.ID,
		Invoice:     "sadfsdafasdfasdfsdfsadf",
		Status:      "completed",
		Description: "Something!",
		Direction:   "inbound",
		Amount:      10000,
	})
	if err != nil {
		return err
	}

	return nil
	// d.Create(&users.User{
	// 	Email:
	// 	Balance:  50000,
	// 	Password: string,
	// })
	// d.Create(&users.User{
	// 	Balance:  0,
	// 	Password: "4321",
	// })
	// d.Create(&users.User{
	// 	Balance:  25000,
	// 	Password: "9876",
	// })

	// invoices := []deposits.Deposit{
	// 	{
	// 		PaymentRequest: "lnbc10u1pwdh735pp5e3p5phcdzjhwc39yvm7jr3w2hvtnwpvdjmptm8829cjcqwvy5clqdqlxycrqvpqwdshgueqvfjhggr0dcsry7qcqzpgyrtvetq6044dtj7x9gf0stpp8c9nrvy2ac22eshyqarnkgv654ts7t3kc09yyjgcw05jeeu8syns5nh5fvc8y7w2aj0a548q6efa55cqy50lfx",
	// 		Description:    "lightningspin",
	// 		SettledAt:      1559140420,
	// 		Amount:         1000,
	// 	},
	// 	{
	// 		PaymentRequest: "lnbc10u1pwdh73lpp5xvlu0jhr3vsj0xyppuw6793qahdcjw56r3mk85jq5mj09w6alpcqdqlxycrqvpqwdshgueqvfjhggr0dcsry7qcqzpg2p6cm8ddmvgvcg3ct2uceseu07tjucvvkdujdds7lw9p6x7g0jy8a6rf3dnaa8yhejarhrzk304vuqjzchvq3pez5sekytn42aa7fvsq75g98j",
	// 		Description:    "lightningspin",
	// 		SettledAt:      1559140420,
	// 		Amount:         1000,
	// 	},
	// 	{
	// 		PaymentRequest: "lnbc10u1pwdh7jdpp5sh0ghtjm32yaqj7vv8dkx6ckx59snflhymyqvknswacey2vjqpcsdqlxycrqvpqwdshgueqvfjhggr0dcsry7qcqzpg42g69hmpc3ftufdtmx6sp27558vjgpmgukd8xlv64rc0g2chfft39vz3gedawt9c9uhqjxma2rzphet4tk2p0jnjlyk5unxxthelvpspxr9uyp",
	// 		Description:    "lightningspin",
	// 		SettledAt:      1559140420,
	// 		Amount:         1000,
	// 	},
	// }
	// for i := range invoices {
	// 	// Although the .Save() function is supposed to UPDATE entries, it will INSERT if a non DB type is entered
	// 	d.Save(&invoices[i])
	// }

	// d.Create(&withdrawals.Withdrawal{
	// 	PaymentRequest: "lnbc10u1pwdh735pp5e3p5phcdzjhwc39yvm7jr3w2hvtnwpvdjmptm8829cjcqwvy5clqdqlxycrqvpqwdshgueqvfjhggr0dcsry7qcqzpgyrtvetq6044dtj7x9gf0stpp8c9nrvy2ac22eshyqarnkgv654ts7t3kc09yyjgcw05jeeu8syns5nh5fvc8y7w2aj0a548q6efa55cqy50lfx",
	// 	Description:    "lightningspin",
	// 	SettledAt:      1559140420,
	// 	Amount:         1000,
	// })
	// d.Create(&withdrawals.Withdrawal{
	// 	PaymentRequest: "lnbc10u1pwdh73lpp5xvlu0jhr3vsj0xyppuw6793qahdcjw56r3mk85jq5mj09w6alpcqdqlxycrqvpqwdshgueqvfjhggr0dcsry7qcqzpg2p6cm8ddmvgvcg3ct2uceseu07tjucvvkdujdds7lw9p6x7g0jy8a6rf3dnaa8yhejarhrzk304vuqjzchvq3pez5sekytn42aa7fvsq75g98j",
	// 	SettledAt:      1559140420,
	// 	Amount:         1000,
	// })
	// d.Create(&withdrawals.Withdrawal{
	// 	PaymentRequest: "lnbc10u1pwdh7jdpp5sh0ghtjm32yaqj7vv8dkx6ckx59snflhymyqvknswacey2vjqpcsdqlxycrqvpqwdshgueqvfjhggr0dcsry7qcqzpg42g69hmpc3ftufdtmx6sp27558vjgpmgukd8xlv64rc0g2chfft39vz3gedawt9c9uhqjxma2rzphet4tk2p0jnjlyk5unxxthelvpspxr9uyp",
	// 	Description:    "lightningspin",
	// 	SettledAt:      1559140420,
	// 	Amount:         1000,
	// 	Fee:            3,
	// })
}

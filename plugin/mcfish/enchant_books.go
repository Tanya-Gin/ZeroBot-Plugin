package mcfish

const (
	enchantBookDailyLureStock  = 2
	enchantBookDailyFavorStock = 1
)

func enchantBookRestockPlan() []store {
	return []store{
		{
			Duration: nextItemID(),
			Name:     "诱钓",
			Number:   enchantBookDailyLureStock,
			Price:    priceList["诱钓"] * discountList["诱钓"] / 100,
			Type:     "treasure",
		},
		{
			Duration: nextItemID(),
			Name:     "海之眷顾",
			Number:   enchantBookDailyFavorStock,
			Price:    priceList["海之眷顾"] * discountList["海之眷顾"] / 100,
			Type:     "treasure",
		},
	}
}

func shouldConsumeEnchantBook(success, atLevelCap bool) bool {
	return success && !atLevelCap
}

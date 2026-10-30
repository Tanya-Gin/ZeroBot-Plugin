package mcfish

import "fmt"

const (
	poleSaleTaxPercent            int64 = 20
	poleRepairPenaltyPercent      int64 = 5
	poleMinimumRepairPercent      int64 = 50
	poleEnchantRecoveryPercent    int64 = 40
	poleEnchantRecoveryCapPercent int64 = 60
)

func poleBaseName(name string) string {
	if name == "初始木竿" {
		return "木竿"
	}
	return name
}

func isPole(name string) bool {
	// 美西螈可装备，但没有鱼竿属性串（耐久/维修/诱钓/眷顾），
	// 配置里的耐久上限只是装备占位，不能走鱼竿估值。
	if name == "美西螈" {
		return false
	}
	return name == "初始木竿" || name == "三叉戟" || durationList[poleBaseName(name)] > 0
}

// calculateItemMarketValue returns the current shop value before sale tax.
// Durable poles use condition-based valuation; other items use list price × discount.
func calculateItemMarketValue(name, other string) (int, error) {
	if isPole(name) {
		return calculatePoleMarketValue(name, other)
	}
	return priceList[name] * discountList[name] / 100, nil
}

// calculatePoleMarketValue returns the current shop value before the sale tax.
// It values a pole by condition, repair history, enchant-book recovery, and the
// current market discount. All calculations are intentionally integer based so
// every displayed price is exactly the price charged by the transaction path.
func calculatePoleMarketValue(name, other string) (int, error) {
	baseName := poleBaseName(name)
	maxDurability, ok := durationList[baseName]
	if !ok || maxDurability <= 0 {
		return 0, fmt.Errorf("unknown pole %q", name)
	}
	basePrice, ok := priceList[baseName]
	if !ok || basePrice < 0 {
		return 0, fmt.Errorf("invalid pole base price for %q", name)
	}

	durable, maintenance, induce, favor, err := parsePoleOther(other)
	if err != nil {
		return 0, err
	}
	if durable > maxDurability {
		return 0, fmt.Errorf("pole durability %d exceeds maximum %d for %q", durable, maxDurability, name)
	}

	base := int64(basePrice)
	enchantRecovery := calculatePoleEnchantRecovery(base, induce, favor)
	conditionedValue := (base + enchantRecovery) * int64(durable) / int64(maxDurability)
	repairPercent := max(poleMinimumRepairPercent, 100-int64(maintenance)*poleRepairPenaltyPercent)
	conditionedValue = conditionedValue * repairPercent / 100

	marketPercent := int64(discountList[baseName])
	if marketPercent < 0 {
		return 0, fmt.Errorf("invalid market discount for %q", name)
	}
	marketValue := conditionedValue * marketPercent / 100
	if marketValue < 0 {
		return 0, fmt.Errorf("negative pole market value for %q", name)
	}
	return int(marketValue), nil
}

func calculatePoleEnchantRecovery(base int64, induce, favor int) int64 {
	bookValue := int64(induce) * int64(priceList["诱钓"]) * int64(discountList["诱钓"]) / 100
	bookValue += int64(favor) * int64(priceList["海之眷顾"]) * int64(discountList["海之眷顾"]) / 100
	recovery := bookValue * poleEnchantRecoveryPercent / 100
	return min(recovery, base*poleEnchantRecoveryCapPercent/100)
}

func calculatePoleSalePayout(name, other string) (int, error) {
	marketValue, err := calculatePoleMarketValue(name, other)
	if err != nil {
		return 0, err
	}
	return marketValue * int(100-poleSaleTaxPercent) / 100, nil
}

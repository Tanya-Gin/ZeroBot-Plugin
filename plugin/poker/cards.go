package poker

// Card describes one card in the canonical poker deck.
type Card struct {
	Name    string
	IsJoker bool
}

// Deck returns a copy of the canonical 54-card deck.
func Deck() []Card {
	deck := make([]Card, 0, len(pokerCardNames))
	for _, name := range pokerCardNames {
		deck = append(deck, Card{Name: name, IsJoker: name == "Lynette.jpg" || name == "Lyney.jpg"})
	}
	return deck
}

// StandardDeck returns a copy of the canonical deck without jokers.
func StandardDeck() []Card {
	deck := Deck()
	standard := make([]Card, 0, 52)
	for _, card := range deck {
		if !card.IsJoker {
			standard = append(standard, card)
		}
	}
	return standard
}

package main

// DeckStyle loads the deck-local visual system. Keeping the stylesheet in
// public/ means gosx-slides copies it into both SPA and single-deck exports.
//
//gosx:island
func DeckStyle(props any) Node {
	return <link rel="stylesheet" href="/public/deck.css"/>
}

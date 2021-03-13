package main

import "log"

func organizer() {
	for {
		go (&Game{
			board: makeBoard(defSize, defStones),
			north: <-waiting,
			south: <-waiting,
		}).Start()
		log.Printf("Start new game")
	}

}

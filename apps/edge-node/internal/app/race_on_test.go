//go:build race

package app

// conRace: el detector de carreras hace todo ~10 veces más lento; los tiempos no valen.
const conRace = true

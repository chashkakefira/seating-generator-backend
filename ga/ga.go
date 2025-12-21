package ga

import (
	"math/rand"
)

type ClassConfig struct {
	Rows    int
	Columns int
}

type Student struct {
	ID                      int
	Name                    string
	PreferredColumns        []int
	PreferredRows           []int
	MedicalPreferredColumns []int
	MedicalPreferredRows    []int
}

type Request struct {
	Students        []Student
	Preferences     [][]int
	Forbidden       [][]int
	ClassConfig     ClassConfig
	PopSize         int
	Generations     int
	CrossOverChance float64
	PriorityWeights PriorityWeights
}

type Response struct {
	SeatID    int
	Row       int
	Column    int
	Student   string
	StudentID int
}

type PriorityWeights struct {
	Medical     float64
	Preferences float64
	Friends     float64
	Enemies     float64
	Fill        float64
}

type Weights struct {
	RowBonus     int64
	PosBonus     int64
	MedPenalty   int64
	FriendBonus  int64
	EnemyPenalty int64
	PrefBonus    int64
}

func calculateWeights(config ClassConfig, pw PriorityWeights) Weights {
	const (
		BASE_FILL   = 1000
		BASE_PREF   = 2000
		BASE_FRIEND = 5000
		BASE_HARD   = 10000
	)

	wFill := int64(pw.Fill * float64(BASE_FILL))
	wPref := int64(pw.Preferences * float64(BASE_PREF))
	wFriend := int64(pw.Friends * float64(BASE_FRIEND))
	wMed := int64(pw.Medical * float64(BASE_HARD))
	wEnemy := int64(pw.Enemies * float64(BASE_HARD))

	return Weights{
		RowBonus: int64(config.Columns) * wFill * 10,
		PosBonus: wFill,

		PrefBonus:    wPref,
		EnemyPenalty: wEnemy,
		FriendBonus:  wFriend,
		MedPenalty:   wMed,
	}
}

func contains(s []int, elem int) bool {
	for _, v := range s {
		if v == elem {
			return true
		}
	}
	return false
}

func fitness(seating []int, students []Student, preferences, forbidden [][]int, config ClassConfig, w Weights) (int64, []int) {
	ignored := make([]int, 0)
	score := 0
	for i, studentIndex := range seating {
		if studentIndex >= len(students) {
			continue
		}
		student := students[studentIndex]
		row := i / config.Columns
		col := i % config.Columns
		score += (config.Rows - row) * int(w.RowBonus)
		score += (config.Columns - col) * int(w.PosBonus)
		if (len(student.PreferredRows) > 0 && contains(student.PreferredRows, row)) || len(student.PreferredColumns) > 0 && contains(student.PreferredColumns, col) {
			score += int(w.PrefBonus) * config.Rows
			ignored = append(ignored, student.ID)
		}

		if len(student.MedicalPreferredColumns) > 0 && !contains(student.MedicalPreferredColumns, col) || len(student.MedicalPreferredRows) > 0 && !contains(student.MedicalPreferredRows, row) {
			score -= int(w.MedPenalty)
		}

	}
	for row := 0; row < config.Rows; row++ {
		for col := 0; col+1 < config.Columns; col++ {
			i := row*config.Columns + col
			if i+1 >= len(seating) || col%2 != 0 || col+1 >= config.Columns {
				continue
			}
			i1 := seating[i]
			i2 := seating[i+1]
			if i1 >= len(students) || i2 >= len(students) {
				continue
			}
			i1ID := students[seating[i]].ID
			i2ID := students[seating[i+1]].ID
			for _, pref := range preferences {
				if (pref[0] == i1ID && pref[1] == i2ID) || (pref[0] == i2ID && pref[1] == i1ID) {
					score += int(w.FriendBonus)
				} else if pref[0] == i1ID || pref[1] == i1ID || pref[0] == i2ID || pref[1] == i2ID {
					ignored = append(ignored, i1ID, i2ID)
				}
			}
			for _, forb := range forbidden {
				if (forb[0] == i1ID && forb[1] == i2ID) || (forb[0] == i2ID && forb[1] == i1ID) {
					score -= int(w.EnemyPenalty)
					ignored = append(ignored, i1ID, i2ID)
				}
			}
		}
	}
	return int64(score), ignored
}

func CrossOver(parent1, parent2 []int) []int {
	N := len(parent1)
	child := make([]int, N)
	used := make(map[int]bool, N)
	start, end := rand.Intn(N), rand.Intn(N)
	if start > end {
		start, end = end, start
	}

	for i := start; i <= end; i++ {
		child[i] = parent1[i]
		used[child[i]] = true
	}

	j := 0
	for i := 0; i < N; i++ {
		if i < start || i > end {
			for j < N && used[parent2[j]] {
				j++
			}
			if j < N {
				child[i] = parent2[j]
				used[child[i]] = true
				j++
			} else {
				for k := 0; k < N; k++ {
					if !used[k] {
						child[i] = k
						used[k] = true
						break
					}
				}
			}
		}
	}
	return child
}

func SwapMutation(seating []int) []int {
	seat := make([]int, len(seating))
	copy(seat, seating)
	i1, i2 := rand.Intn(len(seat)), rand.Intn(len(seat))
	seat[i1], seat[i2] = seat[i2], seat[i1]
	return seat
}

func tournamentSelection(population [][]int, scores []int64, k int) []int {
	bestIdx := -1
	for i := 0; i < k; i++ {
		randIdx := rand.Intn(len(population))
		if bestIdx == -1 || scores[randIdx] > scores[bestIdx] {
			bestIdx = randIdx
		}
	}
	return population[bestIdx]
}
func RunGA(req Request) ([]Response, int64, []int) {
	N := req.ClassConfig.Columns * req.ClassConfig.Rows
	popSize, generations := req.PopSize, req.Generations
	weights := calculateWeights(req.ClassConfig, req.PriorityWeights)
	population := make([][]int, popSize)
	for i := range population {
		population[i] = rand.Perm(N)
	}
	for gen := 0; gen < generations; gen++ {
		scores := make([]int64, popSize)
		ignored := make([][]int, popSize)
		for i, seat := range population {
			scores[i], ignored[i] = fitness(seat, req.Students, req.Preferences, req.Forbidden, req.ClassConfig, weights)

		}
		newPop := make([][]int, popSize)
		iBest := 0
		for j := 1; j < popSize; j++ {
			if scores[j] > scores[iBest] {
				iBest = j
			}
		}
		newPop[0] = make([]int, N)
		copy(newPop[0], population[iBest])
		for i := 1; i < popSize; i++ {
			parent1 := tournamentSelection(population, scores, 3)
			parent2 := tournamentSelection(population, scores, 3)
			child := CrossOver(parent1, parent2)
			if rand.Float64() < req.CrossOverChance {
				child = SwapMutation(child)
			}
			newPop[i] = child
		}
		population = newPop
	}

	iBest := 0
	bestAns, bestIgn := fitness(population[0], req.Students, req.Preferences, req.Forbidden, req.ClassConfig, weights)
	for i, seat := range population {
		Ans, Ign := fitness(seat, req.Students, req.Preferences, req.Forbidden, req.ClassConfig, weights)
		if Ans > bestAns {
			bestAns = Ans
			iBest = i
			bestIgn = Ign
		}
	}
	bestIndices := population[iBest]
	best := make([]int, N)
	for i, idx := range bestIndices {
		if idx < len(req.Students) {
			best[i] = req.Students[idx].ID
		} else {
			best[i] = -1
		}
	}

	response := make([]Response, N)
	for i, studentID := range best {
		row := i / req.ClassConfig.Columns
		col := i % req.ClassConfig.Columns
		if studentID == -1 {
			response[i] = Response{
				SeatID:    i,
				Row:       row,
				Column:    col,
				Student:   "-",
				StudentID: -1,
			}
		} else {
			for _, student := range req.Students {
				if student.ID == best[i] {
					response[i] = Response{
						SeatID:    i,
						Row:       row,
						Column:    col,
						Student:   student.Name,
						StudentID: student.ID,
					}
				}
			}
		}
	}
	return response, bestAns, bestIgn
}

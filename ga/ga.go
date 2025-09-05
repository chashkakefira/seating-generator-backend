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
	Priority        []int
}

type Response struct {
	SeatID    int
	Row       int
	Column    int
	Student   string
	StudentID int
}

func contains(s []int, elem int) bool {
	for _, v := range s {
		if v == elem {
			return true
		}
	}
	return false
}

func fitness(seating []int, students []Student, preferences, forbidden [][]int, config ClassConfig, priority []int) (int64, []int) {
	ignored := make([]int, 0)
	score := 0
	studentMap := make(map[int]Student)
	for _, s := range students {
		studentMap[s.ID] = s
	}
	for i, studentID := range seating {
		student := studentMap[studentID]
		row := i / config.Columns
		col := i % config.Columns
		if (len(student.PreferredRows) > 0 && !contains(student.PreferredRows, row)) || len(student.PreferredColumns) > 0 && !contains(student.PreferredColumns, col) {
			score -= priority[1] * config.Rows
			ignored = append(ignored, studentID)
		} else if len(student.PreferredRows) > 0 || len(student.PreferredColumns) > 0 {
			score += priority[1] * config.Rows
		}

		if len(student.MedicalPreferredColumns) > 0 && !contains(student.MedicalPreferredColumns, col) || len(student.MedicalPreferredRows) > 0 && !contains(student.MedicalPreferredRows, row) {
			score -= priority[0]
		}

	}
	for row := 0; row < config.Rows; row++ {
		for col := 0; col < config.Columns; col++ {
			i := row*config.Columns + col
			if seating[i] < len(students) {
				score += (config.Rows*config.Columns - i)
			}
			if i+1 >= len(seating) || col%2 != 0 || col+1 >= config.Columns {
				continue
			}
			i1 := seating[i]
			i2 := seating[i+1]

			for _, pref := range preferences {
				if (pref[0] == i1 && pref[1] == i2) || (pref[0] == i2 && pref[1] == i1) {
					score += config.Rows * priority[3]
				} else if pref[0] == i1 || pref[1] == i1 || pref[0] == i2 || pref[1] == i2 {
					ignored = append(ignored, i1, i2)
				}
			}
			for _, forb := range forbidden {
				if (forb[0] == i1 && forb[1] == i2) || (forb[0] == i2 && forb[1] == i1) {
					score -= config.Rows * priority[2]
					ignored = append(ignored, i1, i2)
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

func RunGA(req Request) ([]Response, int64, []int) {
	N := req.ClassConfig.Columns * req.ClassConfig.Rows
	popSize, generations := req.PopSize, req.Generations
	priorities := make([]int, 4)
	for i := range req.Priority {
		priorities[req.Priority[i]] = i + 1
	}
	population := make([][]int, popSize)
	for i := range population {
		population[i] = rand.Perm(N)
	}
	for gen := 0; gen < generations; gen++ {
		scores := make([]int64, popSize)
		ignored := make([][]int, popSize)
		for i, seat := range population {
			scores[i], ignored[i] = fitness(seat, req.Students, req.Preferences, req.Forbidden, req.ClassConfig, priorities)

		}
		newPop := make([][]int, popSize)
		for i := 0; i < popSize/2; i++ {
			iBest := 0
			for j := 1; j < popSize; j++ {
				if scores[j] > scores[iBest] {
					iBest = j
				}
			}
			newPop[i] = make([]int, N)
			copy(newPop[i], population[iBest])
			scores[iBest] = -1e9
		}
		for i := popSize / 2; i < popSize; i++ {
			parent1, parent2 := newPop[rand.Intn(popSize/2)], newPop[rand.Intn(popSize/2)]
			child := CrossOver(parent1, parent2)
			if rand.Float64() < req.CrossOverChance {
				child = SwapMutation(child)
			}
			newPop[i] = child
		}
		population = newPop
	}

	iBest := 0
	bestAns, bestIgn := fitness(population[0], req.Students, req.Preferences, req.Forbidden, req.ClassConfig, priorities)
	for i, seat := range population {
		Ans, Ign := fitness(seat, req.Students, req.Preferences, req.Forbidden, req.ClassConfig, priorities)
		if Ans > bestAns {
			bestAns = Ans
			iBest = i
			bestIgn = Ign
		}
	}
	best := population[iBest]

	response := make([]Response, N)
	for i, studentID := range best {
		row := i / req.ClassConfig.Columns
		col := i % req.ClassConfig.Columns
		if studentID > len(req.Students)-1 {
			response[i] = Response{
				SeatID:    i,
				Row:       row,
				Column:    col,
				Student:   "-",
				StudentID: -1,
			}
		} else {
			response[i] = Response{
				SeatID:    i,
				Row:       row,
				Column:    col,
				Student:   req.Students[studentID].Name,
				StudentID: req.Students[studentID].ID,
			}
		}
	}
	return response, bestAns, bestIgn
}

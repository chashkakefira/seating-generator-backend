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
	RowBonus     int
	PosBonus     int
	MedPenalty   int
	FriendBonus  int
	EnemyPenalty int
	PrefBonus    int
}

func calculateWeights(config ClassConfig, pw PriorityWeights) Weights {
	const (
		BASE_FILL   = 5000
		BASE_PREF   = 8000
		BASE_FRIEND = 10000
		BASE_HARD   = 15000
	)

	wFill := int(pw.Fill * float64(BASE_FILL))
	wPref := int(pw.Preferences * float64(BASE_PREF))
	wFriend := int(pw.Friends * float64(BASE_FRIEND))
	wMed := int(pw.Medical * float64(BASE_HARD))
	wEnemy := int(pw.Enemies * float64(BASE_HARD))
	return Weights{
		RowBonus: wFill * 2,
		PosBonus: wFill,

		PrefBonus:    wPref,
		EnemyPenalty: wEnemy,
		FriendBonus:  wFriend,
		MedPenalty:   wMed,
	}
}

type SocialMap map[int]map[int]bool

func contains(s []int, elem int) bool {
	for _, v := range s {
		if v == elem {
			return true
		}
	}
	return false
}

func abs(num int) int {
	if num < 0 {
		return -num
	}
	return num
}

func buildSocialMap(req Request) (SocialMap, SocialMap) {
	friends := make(SocialMap)
	enemies := make(SocialMap)
	for _, pair := range req.Preferences {
		if friends[pair[0]] == nil {
			friends[pair[0]] = make(map[int]bool)
		}
		if friends[pair[1]] == nil {
			friends[pair[1]] = make(map[int]bool)
		}
		friends[pair[0]][pair[1]] = true
		friends[pair[1]][pair[0]] = true
	}
	for _, pair := range req.Forbidden {
		if enemies[pair[0]] == nil {
			enemies[pair[0]] = make(map[int]bool)
		}
		if enemies[pair[1]] == nil {
			enemies[pair[1]] = make(map[int]bool)
		}
		enemies[pair[0]][pair[1]] = true
		enemies[pair[1]][pair[0]] = true
	}
	return friends, enemies
}

func checkMed(student Student, row, col int, w Weights) int {
	if len(student.MedicalPreferredColumns) > 0 && !contains(student.MedicalPreferredColumns, col) || len(student.MedicalPreferredRows) > 0 && !contains(student.MedicalPreferredRows, row) {
		return -w.MedPenalty
	}
	return 0
}

func checkPref(student Student, row, col int, w Weights, config ClassConfig) int {
	if (len(student.PreferredRows) > 0 && contains(student.PreferredRows, row)) || len(student.PreferredColumns) > 0 && contains(student.PreferredColumns, col) {
		return w.PrefBonus * config.Rows
	}
	return 0
}

func checkFriends(student Student, seating []int, row, col int, w Weights, config ClassConfig, friends SocialMap, students []Student) int {
	score := 0
	for dcol := -1; dcol <= 1; dcol++ {
		for drow := -1; drow <= 1; drow++ {
			if dcol == 0 && drow == 0 {
				continue
			}
			nrow, ncol := row+drow, col+dcol
			if nrow < 0 || nrow >= config.Rows || ncol < 0 || ncol >= config.Columns {
				continue
			}
			neighborIdx := seating[nrow*config.Columns+ncol]
			if neighborIdx < 0 || neighborIdx >= len(students) {
				continue
			}
			neighborID := students[neighborIdx].ID
			if friends[student.ID][neighborID] {
				if drow == 0 && col/2 == ncol/2 {
					score += w.FriendBonus * 3
				} else if abs(drow) == 1 && col/2 == ncol/2 {
					score += w.FriendBonus * 2
				} else {
					score += w.FriendBonus
				}
			}
		}
	}
	return score
}

func checkEnemies(student Student, seating []int, row, col int, w Weights, config ClassConfig, enemies SocialMap, students []Student) int {
	penalty := 0
	for dcol := -1; dcol <= 1; dcol++ {
		for drow := -1; drow <= 1; drow++ {
			if dcol == 0 && drow == 0 {
				continue
			}
			nrow, ncol := row+drow, col+dcol
			if nrow < 0 || nrow >= config.Rows || ncol < 0 || ncol >= config.Columns {
				continue
			}
			neighborIdx := seating[nrow*config.Columns+ncol]
			if neighborIdx < 0 || neighborIdx >= len(students) {
				continue
			}
			neighborID := students[neighborIdx].ID
			if enemies[student.ID][neighborID] {
				if drow == 0 && col/2 == ncol/2 {
					penalty -= w.EnemyPenalty * 10
				} else if abs(drow) == 1 && col/2 == ncol/2 {
					penalty -= w.EnemyPenalty * 5
				} else {
					penalty -= w.EnemyPenalty * 2
				}
			}
		}
	}
	return penalty
}

func studentsSatisfaction(seating []int, row, col, studentIndex int, w Weights, config ClassConfig, friends, enemies SocialMap, students []Student) int {
	score := 0
	if studentIndex >= len(students) || studentIndex < 0 {
		score -= (config.Rows - row) * w.RowBonus
		return score
	}
	student := students[studentIndex]
	score += (config.Rows - row) * w.RowBonus
	score += (config.Columns - col) * w.PosBonus
	score += checkMed(student, row, col, w)
	score += checkPref(student, row, col, w, config)
	score += checkFriends(student, seating, row, col, w, config, friends, students)
	score += checkEnemies(student, seating, row, col, w, config, enemies, students)
	return score
}

func fitness(seating []int, students []Student, preferences, forbidden [][]int, config ClassConfig, w Weights, friends SocialMap, enemies SocialMap) int {
	score := 0
	for i, studentIndex := range seating {
		score += studentsSatisfaction(seating, i/config.Columns, i%config.Columns, studentIndex, w, config, friends, enemies, students)
	}
	return score
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

func tournamentSelection(population [][]int, scores []int, k int) []int {
	bestIdx := -1
	for i := 0; i < k; i++ {
		randIdx := rand.Intn(len(population))
		if bestIdx == -1 || scores[randIdx] > scores[bestIdx] {
			bestIdx = randIdx
		}
	}
	return population[bestIdx]
}
func RunGA(req Request) ([]Response, int) {
	N := req.ClassConfig.Columns * req.ClassConfig.Rows
	popSize, generations := req.PopSize, req.Generations
	weights := calculateWeights(req.ClassConfig, req.PriorityWeights)
	population := make([][]int, popSize)
	for i := range population {
		population[i] = rand.Perm(N)
	}
	friends, enemies := buildSocialMap(req)
	for gen := 0; gen < generations; gen++ {
		scores := make([]int, popSize)
		for i, seat := range population {
			scores[i] = fitness(seat, req.Students, req.Preferences, req.Forbidden, req.ClassConfig, weights, friends, enemies)

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
	bestAns := fitness(population[0], req.Students, req.Preferences, req.Forbidden, req.ClassConfig, weights, friends, enemies)
	for i, seat := range population {
		Ans := fitness(seat, req.Students, req.Preferences, req.Forbidden, req.ClassConfig, weights, friends, enemies)
		if Ans > bestAns {
			bestAns = Ans
			iBest = i
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
	return response, bestAns
}

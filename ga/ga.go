package ga

import (
	"math/rand"
)

type ClassConfig struct {
	Rows     int
	Columns  int
	deskType string
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

type SatisfactionDetails struct {
	Total      float64
	Medical    float64
	Friends    float64
	Enemies    float64
	Pref       float64
	RowBonus   float64
	Level      float64
	Complaints []string
}

type Response struct {
	SeatID       int
	Row          int
	Column       int
	Student      string
	StudentID    int
	Satisfaction SatisfactionDetails
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
	MedPenalty   int
	FriendBonus  int
	EnemyPenalty int
	PrefBonus    int
}

func calculateWeights(pw PriorityWeights) Weights {
	return Weights{
		RowBonus:     int(pw.Fill),
		PrefBonus:    int(pw.Preferences),
		FriendBonus:  int(pw.Friends),
		MedPenalty:   int(pw.Medical),
		EnemyPenalty: int(pw.Enemies),
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

func scorePosition(row, totalRows int) float64 {
	if totalRows <= 1 {
		return 1.0
	}
	return 1.0 - (float64(row) / float64(totalRows-1))
}

func isSameDesk(col1, col2 int, seatType string) bool {
	seatsPerDesk := 2
	if seatType == "single" {
		seatsPerDesk = 1
	}
	return col1/seatsPerDesk == col2/seatsPerDesk
}

func checkMed(student Student, row, col int) float64 {
	if len(student.MedicalPreferredColumns) == 0 && len(student.MedicalPreferredRows) == 0 {
		return 0.0
	}
	score := 0.0
	if len(student.MedicalPreferredColumns) > 0 && contains(student.MedicalPreferredColumns, col) {
		score += 0.5
	}
	if len(student.MedicalPreferredRows) > 0 && contains(student.MedicalPreferredRows, row) {
		score += 0.5
	}
	if score == 0 {
		return -1.0
	}
	return score
}

func checkPref(student Student, row, col int, config ClassConfig) float64 {
	score := 0.0
	if len(student.PreferredRows) > 0 && contains(student.PreferredRows, row) {
		score += 1
	}
	if len(student.PreferredColumns) > 0 && contains(student.PreferredColumns, col) {
		score += 1
	}
	if len(student.PreferredRows) == 0 && len(student.PreferredColumns) == 0 {
		return 1.0
	}
	return score / 2.0
}

func checkFriends(student Student, seating []int, row, col int, config ClassConfig, friends SocialMap, students []Student) float64 {
	score := 0.0
	maxScore := 1.5
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
				if drow == 0 && isSameDesk(col, ncol, config.deskType) {
					score += 1
				} else if abs(drow) == 1 && abs(dcol) == 0 {
					score += 0.7
				} else if abs(drow) == 0 && abs(dcol) == 1 {
					score += 0.5
				} else {
					score += 0.2
				}
			}
		}
	}
	if score > maxScore {
		score = maxScore
	}
	return score / 1.5
}

func checkEnemies(student Student, seating []int, row, col int, config ClassConfig, enemies SocialMap, students []Student) float64 {
	penalty := 0.0
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
				if drow == 0 && isSameDesk(col, ncol, config.deskType) {
					penalty += 1
				} else if abs(drow) == 1 && abs(dcol) == 0 {
					penalty += 0.7
				} else if abs(drow) == 0 && abs(dcol) == 1 {
					penalty += 0.5
				} else {
					penalty += 0.2
				}
			}
		}
	}
	return penalty
}

func studentsSatisfaction(seating []int, row, col, studentIndex int, w Weights, config ClassConfig, friends, enemies SocialMap, students []Student) int {
	const Base = 100
	if studentIndex >= len(students) || studentIndex < 0 {
		return 0
	}
	student := students[studentIndex]

	fScore := checkFriends(student, seating, row, col, config, friends, students)
	mScore := checkMed(student, row, col)
	pScore := checkPref(student, row, col, config)
	rScore := scorePosition(row, config.Rows)
	ePenalty := checkEnemies(student, seating, row, col, config, enemies, students)

	res := (fScore * float64(w.FriendBonus)) +
		(pScore * float64(w.PrefBonus)) +
		(rScore * float64(w.RowBonus))

	res -= (ePenalty * float64(w.EnemyPenalty) * 5.0)

	if mScore > 0 {
		res += float64(w.MedPenalty)
	} else if mScore < 0 {
		res -= float64(w.MedPenalty) * 10.0
	}

	return int(res * Base)
}

func getSatisfactionDetails(seating []int, row, col, studentIndex int, w Weights, config ClassConfig, friends, enemies SocialMap, students []Student) SatisfactionDetails {
	var details SatisfactionDetails
	student := students[studentIndex]

	mScore := checkMed(student, row, col)
	pScore := checkPref(student, row, col, config)
	fScore := checkFriends(student, seating, row, col, config, friends, students)
	ePenalty := checkEnemies(student, seating, row, col, config, enemies, students)
	rScore := scorePosition(row, config.Rows)

	details.Medical = 0
	if mScore > 0 {
		details.Medical = float64(w.MedPenalty)
	}
	if mScore < 0 {
		details.Medical = -float64(w.MedPenalty) * 10.0
	}

	details.Pref = pScore * float64(w.PrefBonus)
	details.Friends = fScore * float64(w.FriendBonus)
	details.RowBonus = rScore * float64(w.RowBonus)
	details.Enemies = ePenalty * float64(w.EnemyPenalty) * -5.0

	details.Total = details.Medical + details.Pref + details.Friends + details.RowBonus + details.Enemies

	if mScore < 0 {
		details.Complaints = append(details.Complaints, "Нарушены медицинские показания")
	}
	if ePenalty > 0 {
		details.Complaints = append(details.Complaints, "Рядом сидит нежелательный человек")
	}

	maxPossible := float64(w.PrefBonus + w.FriendBonus + w.RowBonus)
	if maxPossible <= 0 {
		details.Level = 1.0
	} else {
		currentGain := (pScore * float64(w.PrefBonus)) +
			(fScore * float64(w.FriendBonus)) +
			(rScore * float64(w.RowBonus))
		details.Level = currentGain / maxPossible
	}

	if mScore < 0 || ePenalty > 0 {
		details.Level = 0.0
	}

	return details
}
func fitness(seating []int, students []Student, config ClassConfig, w Weights, friends SocialMap, enemies SocialMap) int {
	score := 0
	for i, studentIndex := range seating {
		if studentIndex >= len(students) || studentIndex < 0 {
			score -= (config.Rows - i/config.Columns) * w.RowBonus * 10
			continue
		}
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

func localSearch(seating []int, students []Student, config ClassConfig, w Weights, friends, enemies SocialMap) []int {
	current := make([]int, len(seating))
	copy(current, seating)
	currentFit := fitness(current, students, config, w, friends, enemies)

	for i := 0; i < 20; i++ {
		idx1 := rand.Intn(len(current))
		idx2 := rand.Intn(len(current))

		current[idx1], current[idx2] = current[idx2], current[idx1]
		newFit := fitness(current, students, config, w, friends, enemies)

		if newFit > currentFit {
			currentFit = newFit
		} else {
			current[idx1], current[idx2] = current[idx2], current[idx1]
		}
	}
	return current
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
	cp := make([]int, len(population[bestIdx]))
	copy(cp, population[bestIdx])
	return cp
}
func RunGA(req Request) ([]Response, int) {
	N := req.ClassConfig.Columns * req.ClassConfig.Rows
	popSize, generations := req.PopSize, req.Generations
	weights := calculateWeights(req.PriorityWeights)
	population := make([][]int, popSize)
	for i := range population {
		population[i] = rand.Perm(N)
	}
	friends, enemies := buildSocialMap(req)
	for gen := 0; gen < generations; gen++ {
		scores := make([]int, popSize)
		iBest := 0
		for i := range population {
			scores[i] = fitness(population[i], req.Students, req.ClassConfig, weights, friends, enemies)
			if scores[i] > scores[iBest] {
				iBest = i
			}
		}

		newPop := make([][]int, popSize)

		bestCopy := make([]int, N)
		copy(bestCopy, population[iBest])
		newPop[0] = localSearch(bestCopy, req.Students, req.ClassConfig, weights, friends, enemies)

		for i := 1; i < popSize; i++ {
			p1 := tournamentSelection(population, scores, 3)
			p2 := tournamentSelection(population, scores, 3)

			child := CrossOver(p1, p2)

			if rand.Float64() < 0.2 {
				child = SwapMutation(child)
			}
			newPop[i] = child
		}
		population = newPop
	}

	iBest := 0
	bestAns := fitness(population[0], req.Students, req.ClassConfig, weights, friends, enemies)
	for i, seat := range population {
		Ans := fitness(seat, req.Students, req.ClassConfig, weights, friends, enemies)
		if Ans > bestAns {
			bestAns = Ans
			iBest = i
		}
	}
	bestIndices := population[iBest]
	best := make([]int, N)
	studentLookup := make(map[int]Student)
	for _, s := range req.Students {
		studentLookup[s.ID] = s
	}
	for i, idx := range bestIndices {
		if idx < len(req.Students) {
			best[i] = req.Students[idx].ID
		} else {
			best[i] = -1
		}
	}

	response := make([]Response, N)
	for i, studentIdx := range bestIndices {
		row := i / req.ClassConfig.Columns
		col := i % req.ClassConfig.Columns

		if studentIdx >= len(req.Students) || studentIdx < 0 {
			response[i] = Response{
				SeatID: i, Row: row, Column: col,
				Student: "-", StudentID: -1,
			}
			continue
		}

		details := getSatisfactionDetails(bestIndices, row, col, studentIdx, weights, req.ClassConfig, friends, enemies, req.Students)

		student := req.Students[studentIdx]
		response[i] = Response{
			SeatID:       i,
			Row:          row,
			Column:       col,
			Student:      student.Name,
			StudentID:    student.ID,
			Satisfaction: details,
		}
	}
	return response, bestAns
}

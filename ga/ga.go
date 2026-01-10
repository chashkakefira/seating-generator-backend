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

type optStudent struct {
	Student
	index int
	pCols map[int]bool
	pRows map[int]bool
	mCols map[int]bool
	mRows map[int]bool
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
	RowBonus     float64
	MedPenalty   float64
	FriendBonus  float64
	EnemyPenalty float64
	PrefBonus    float64
}

func calculateWeights(pw PriorityWeights) Weights {
	return Weights{
		RowBonus:     float64(pw.Fill),
		PrefBonus:    float64(pw.Preferences),
		FriendBonus:  float64(pw.Friends),
		MedPenalty:   float64(pw.Medical),
		EnemyPenalty: float64(pw.Enemies),
	}
}

type SocialMap []bool

func abs(num int) int {
	if num < 0 {
		return -num
	}
	return num
}

func buildSocialMap(req Request, idToIndex map[int]int) (SocialMap, SocialMap) {
	n := len(req.Students)
	friends := make(SocialMap, n*n)
	enemies := make(SocialMap, n*n)
	for _, pair := range req.Preferences {
		idx1, ok1 := idToIndex[pair[0]]
		idx2, ok2 := idToIndex[pair[1]]
		if ok1 && ok2 {
			friends[idx1*n+idx2] = true
			friends[idx2*n+idx1] = true
		}
	}
	for _, pair := range req.Forbidden {
		idx1, ok1 := idToIndex[pair[0]]
		idx2, ok2 := idToIndex[pair[1]]
		if ok1 && ok2 {
			enemies[idx1*n+idx2] = true
			enemies[idx2*n+idx1] = true
		}
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

func checkMed(student optStudent, row, col int) float64 {
	if len(student.mCols) == 0 && len(student.mRows) == 0 {
		return 0.0
	}
	rowMatch, colMatch := student.mRows[row], student.mCols[col]
	if len(student.mRows) > 0 && len(student.mCols) > 0 {
		if rowMatch && colMatch {
			return 1.0
		}
		if rowMatch || colMatch {
			return 0.1
		}
		return -1.0
	}
	if rowMatch || colMatch {
		return 1.0
	}
	return -1.0
}

func checkPref(student optStudent, row, col int) float64 {
	score := 0.0
	if student.pRows[row] {
		score += 1.0
	}
	if student.pCols[col] {
		score += 1.0
	}
	if len(student.pRows) == 0 && len(student.pCols) == 0 {
		return 1.0
	}
	return score / 2.0
}

func checkFriends(student optStudent, seating []int, row, col int, config ClassConfig, friends SocialMap, students []optStudent) float64 {
	score := 0.0
	maxScore := 1.5
	n := len(students)
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
			if neighborIdx < 0 || neighborIdx >= n {
				continue
			}
			if friends[student.index*n+neighborIdx] {
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

func checkEnemies(student optStudent, seating []int, row, col int, config ClassConfig, enemies SocialMap, students []optStudent) float64 {
	penalty := 0.0
	n := len(students)
	for dcol := -2; dcol <= 2; dcol++ {
		for drow := -2; drow <= 2; drow++ {
			if dcol == 0 && drow == 0 {
				continue
			}
			nrow, ncol := row+drow, col+dcol
			if nrow < 0 || nrow >= config.Rows || ncol < 0 || ncol >= config.Columns {
				continue
			}
			neighborIdx := seating[nrow*config.Columns+ncol]
			if neighborIdx < 0 || neighborIdx >= n {
				continue
			}
			if enemies[student.index*n+neighborIdx] {
				distRow := abs(drow)
				distCol := abs(dcol)
				if drow == 0 && isSameDesk(col, ncol, config.deskType) {
					penalty += 1.0
				} else if distRow <= 1 && distCol <= 1 {
					penalty += 0.8
				} else {
					penalty += 0.5
				}
			}
		}
	}
	return penalty
}

func studentsSatisfaction(seating []int, row, col, studentIndex int, w Weights, config ClassConfig, friends, enemies SocialMap, students []optStudent) float64 {
	const Base = 100
	if studentIndex >= len(students) || studentIndex < 0 {
		return 0
	}
	student := students[studentIndex]

	fScore := checkFriends(student, seating, row, col, config, friends, students)
	mScore := checkMed(student, row, col)
	pScore := checkPref(student, row, col)
	rScore := scorePosition(row, config.Rows)
	ePenalty := checkEnemies(student, seating, row, col, config, enemies, students)

	res := (fScore * w.FriendBonus) +
		(pScore * w.PrefBonus) +
		(rScore * w.RowBonus)

	res -= (ePenalty * w.EnemyPenalty * 5.0)

	if mScore > 0 {
		res += mScore * w.MedPenalty
	} else if mScore < 0 {
		res -= w.MedPenalty * 20.0
	}

	return res * Base
}

func getSatisfactionDetails(seating []int, row, col, studentIndex int, w Weights, config ClassConfig, friends, enemies SocialMap, students []optStudent) SatisfactionDetails {
	var details SatisfactionDetails
	student := students[studentIndex]

	mScore := checkMed(student, row, col)
	pScore := checkPref(student, row, col)
	fScore := checkFriends(student, seating, row, col, config, friends, students)
	ePenalty := checkEnemies(student, seating, row, col, config, enemies, students)
	rScore := scorePosition(row, config.Rows)

	details.Medical = 0
	if mScore > 0 {
		details.Medical = mScore * w.MedPenalty
	} else if mScore < 0 {
		details.Medical = -w.MedPenalty * 10.0
	}

	details.Pref = pScore * w.PrefBonus
	details.Friends = fScore * w.FriendBonus
	details.RowBonus = rScore * w.RowBonus
	details.Enemies = ePenalty * w.EnemyPenalty * -5.0

	details.Total = details.Medical + details.Pref + details.Friends + details.RowBonus + details.Enemies

	maxPossible := w.MedPenalty + w.PrefBonus + w.FriendBonus + w.RowBonus

	if maxPossible <= 0 {
		details.Level = 1.0
	} else {
		currentGood := 0.0
		if mScore > 0 {
			currentGood += details.Medical
		}
		currentGood += (pScore * w.PrefBonus) +
			(fScore * w.FriendBonus) +
			(rScore * w.RowBonus)

		details.Level = currentGood / maxPossible

		if mScore < 0 {
			details.Level = 0.0
			details.Complaints = append(details.Complaints, "Нарушены медицинские показания")
		}
		if ePenalty > 0 {
			details.Level *= 0.5
			details.Complaints = append(details.Complaints, "Рядом сидит нежелательный человек")
		}
	}
	return details
}

func fitness(seating []int, students []optStudent, config ClassConfig, w Weights, friends SocialMap, enemies SocialMap) float64 {
	score := 0.0
	for i, studentIndex := range seating {
		if studentIndex >= len(students) || studentIndex < 0 {
			score -= float64(config.Rows-i/config.Columns) * w.RowBonus * 10
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
			}
		}
	}
	return child
}

func localSearch(seating []int, students []optStudent, config ClassConfig, w Weights, friends, enemies SocialMap) []int {
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

func tournamentSelection(population [][]int, scores []float64, k int) []int {
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

func RunGA(req Request) ([]Response, float64) {
	N := req.ClassConfig.Columns * req.ClassConfig.Rows
	popSize, generations := req.PopSize, req.Generations
	weights := calculateWeights(req.PriorityWeights)

	idToIndex := make(map[int]int)
	opt := make([]optStudent, len(req.Students))
	for i, s := range req.Students {
		idToIndex[s.ID] = i
		m := func(sl []int) map[int]bool {
			r := make(map[int]bool)
			for _, v := range sl {
				r[v] = true
			}
			return r
		}
		opt[i] = optStudent{
			Student: s,
			index:   i,
			pCols:   m(s.PreferredColumns),
			pRows:   m(s.PreferredRows),
			mCols:   m(s.MedicalPreferredColumns),
			mRows:   m(s.MedicalPreferredRows),
		}
	}

	population := make([][]int, popSize)
	for i := range population {
		population[i] = rand.Perm(N)
	}
	friends, enemies := buildSocialMap(req, idToIndex)

	for gen := 0; gen < generations; gen++ {
		scores := make([]float64, popSize)
		for i := range population {
			scores[i] = fitness(population[i], opt, req.ClassConfig, weights, friends, enemies)
		}
		iBest := 0
		for i := range scores {
			if scores[i] > scores[iBest] {
				iBest = i
			}
		}
		newPop := make([][]int, popSize)
		bestCopy := make([]int, N)
		copy(bestCopy, population[iBest])
		newPop[0] = localSearch(bestCopy, opt, req.ClassConfig, weights, friends, enemies)
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

	bestIdx := 0
	bestAns := fitness(population[0], opt, req.ClassConfig, weights, friends, enemies)
	for i, seat := range population {
		Ans := fitness(seat, opt, req.ClassConfig, weights, friends, enemies)
		if Ans > bestAns {
			bestAns = Ans
			bestIdx = i
		}
	}

	bestIndices := population[bestIdx]
	response := make([]Response, N)
	for i, studentIdx := range bestIndices {
		row, col := i/req.ClassConfig.Columns, i%req.ClassConfig.Columns
		if studentIdx >= len(req.Students) || studentIdx < 0 {
			response[i] = Response{SeatID: i, Row: row, Column: col, Student: "-", StudentID: -1}
			continue
		}
		response[i] = Response{
			SeatID:       i,
			Row:          row,
			Column:       col,
			Student:      opt[studentIdx].Name,
			StudentID:    opt[studentIdx].ID,
			Satisfaction: getSatisfactionDetails(bestIndices, row, col, studentIdx, weights, req.ClassConfig, friends, enemies, opt),
		}
	}
	return response, bestAns
}

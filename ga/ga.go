package ga

import (
	"math/rand"
	"runtime"
	"sync"
	"time"
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

func checkFriends(studentIdx int, seating []int, row, col int, config ClassConfig, friends SocialMap, n int) float64 {
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
			if neighborIdx < 0 || neighborIdx >= n {
				continue
			}
			if friends[studentIdx*n+neighborIdx] {
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

func checkEnemies(studentIdx int, seating []int, row, col int, config ClassConfig, enemies SocialMap, n int) float64 {
	penalty := 0.0
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
			if enemies[studentIdx*n+neighborIdx] {
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

func fitness(seating []int, config ClassConfig, w Weights, friends SocialMap, enemies SocialMap, staticScores []float64, nStudents int) float64 {
	score := 0.0
	for i, studentIdx := range seating {
		if studentIdx < 0 || studentIdx >= nStudents {
			score -= float64(config.Rows-i/config.Columns) * w.RowBonus * 10
			continue
		}
		row, col := i/config.Columns, i%config.Columns
		fScore := checkFriends(studentIdx, seating, row, col, config, friends, nStudents)
		ePenalty := checkEnemies(studentIdx, seating, row, col, config, enemies, nStudents)

		sScore := (fScore * w.FriendBonus * 100.0) - (ePenalty * w.EnemyPenalty * 5.0 * 100.0)
		sScore += staticScores[studentIdx*config.Rows*config.Columns+i]

		score += sScore
	}
	return score
}

func CrossOver(r *rand.Rand, parent1, parent2, child []int, used []bool) {
	N := len(parent1)
	for i := 0; i < N; i++ {
		used[i] = false
	}
	start, end := r.Intn(N), r.Intn(N)
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
}

func localSearch(r *rand.Rand, seating []int, config ClassConfig, w Weights, friends, enemies SocialMap, staticScores []float64, nStudents int) {
	currentFit := fitness(seating, config, w, friends, enemies, staticScores, nStudents)
	for i := 0; i < 20; i++ {
		idx1 := r.Intn(len(seating))
		idx2 := r.Intn(len(seating))
		seating[idx1], seating[idx2] = seating[idx2], seating[idx1]
		newFit := fitness(seating, config, w, friends, enemies, staticScores, nStudents)
		if newFit > currentFit {
			currentFit = newFit
		} else {
			seating[idx1], seating[idx2] = seating[idx2], seating[idx1]
		}
	}
}

func SwapMutation(r *rand.Rand, seating []int) {
	i1, i2 := r.Intn(len(seating)), r.Intn(len(seating))
	seating[i1], seating[i2] = seating[i2], seating[i1]
}

func tournamentSelection(r *rand.Rand, population [][]int, scores []float64, k int) int {
	bestIdx := r.Intn(len(population))
	for i := 1; i < k; i++ {
		randIdx := r.Intn(len(population))
		if scores[randIdx] > scores[bestIdx] {
			bestIdx = randIdx
		}
	}
	return bestIdx
}

func RunGA(req Request) ([]Response, float64) {
	N := req.ClassConfig.Columns * req.ClassConfig.Rows
	nStudents := len(req.Students)
	popSize, generations := req.PopSize, req.Generations
	weights := calculateWeights(req.PriorityWeights)
	numCPU := runtime.NumCPU()

	idToIndex := make(map[int]int)
	opt := make([]optStudent, nStudents)
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
			Student: s, index: i,
			pCols: m(s.PreferredColumns), pRows: m(s.PreferredRows),
			mCols: m(s.MedicalPreferredColumns), mRows: m(s.MedicalPreferredRows),
		}
	}

	staticScores := make([]float64, nStudents*N)
	for i := 0; i < nStudents; i++ {
		for seatIdx := 0; seatIdx < N; seatIdx++ {
			r, c := seatIdx/req.ClassConfig.Columns, seatIdx%req.ClassConfig.Columns
			mScore := checkMed(opt[i], r, c)
			pScore := checkPref(opt[i], r, c)
			rScore := scorePosition(r, req.ClassConfig.Rows)

			val := (pScore * weights.PrefBonus) + (rScore * weights.RowBonus)
			if mScore > 0 {
				val += mScore * weights.MedPenalty
			} else if mScore < 0 {
				val -= weights.MedPenalty * 20.0
			}
			staticScores[i*N+seatIdx] = val * 100.0
		}
	}

	rands := make([]*rand.Rand, numCPU)
	for i := 0; i < numCPU; i++ {
		rands[i] = rand.New(rand.NewSource(time.Now().UnixNano() + int64(i)))
	}

	population := make([][]int, popSize)
	for i := range population {
		population[i] = rands[0].Perm(N)
	}
	friends, enemies := buildSocialMap(req, idToIndex)

	newPop := make([][]int, popSize)
	for i := range newPop {
		newPop[i] = make([]int, N)
	}
	usedBufs := make([][]bool, popSize)
	for i := range usedBufs {
		usedBufs[i] = make([]bool, N)
	}

	scores := make([]float64, popSize)
	var wg sync.WaitGroup

	for gen := 0; gen < generations; gen++ {
		chunkSize := (popSize + numCPU - 1) / numCPU
		for w := 0; w < numCPU; w++ {
			start, end := w*chunkSize, (w+1)*chunkSize
			if start >= popSize {
				break
			}
			if end > popSize {
				end = popSize
			}
			wg.Add(1)
			go func(s, e int) {
				defer wg.Done()
				for i := s; i < e; i++ {
					scores[i] = fitness(population[i], req.ClassConfig, weights, friends, enemies, staticScores, nStudents)
				}
			}(start, end)
		}
		wg.Wait()

		iBest := 0
		for i := 1; i < popSize; i++ {
			if scores[i] > scores[iBest] {
				iBest = i
			}
		}

		copy(newPop[0], population[iBest])
		localSearch(rands[0], newPop[0], req.ClassConfig, weights, friends, enemies, staticScores, nStudents)

		for w := 0; w < numCPU; w++ {
			start, end := w*chunkSize, (w+1)*chunkSize
			if start == 0 {
				start = 1
			}
			if start >= popSize {
				break
			}
			if end > popSize {
				end = popSize
			}
			wg.Add(1)
			go func(s, e int, r *rand.Rand) {
				defer wg.Done()
				for i := s; i < e; i++ {
					p1Idx := tournamentSelection(r, population, scores, 3)
					p2Idx := tournamentSelection(r, population, scores, 3)
					CrossOver(r, population[p1Idx], population[p2Idx], newPop[i], usedBufs[i])
					if r.Float64() < 0.2 {
						SwapMutation(r, newPop[i])
					}
				}
			}(start, end, rands[w])
		}
		wg.Wait()

		population, newPop = newPop, population
	}

	bestIdx := 0
	bestAns := fitness(population[0], req.ClassConfig, weights, friends, enemies, staticScores, nStudents)
	for i := 1; i < popSize; i++ {
		Ans := fitness(population[i], req.ClassConfig, weights, friends, enemies, staticScores, nStudents)
		if Ans > bestAns {
			bestAns = Ans
			bestIdx = i
		}
	}

	bestIndices := population[bestIdx]
	response := make([]Response, N)
	for i, studentIdx := range bestIndices {
		row, col := i/req.ClassConfig.Columns, i%req.ClassConfig.Columns
		if studentIdx >= nStudents || studentIdx < 0 {
			response[i] = Response{SeatID: i, Row: row, Column: col, Student: "-", StudentID: -1}
			continue
		}
		response[i] = Response{
			SeatID: i, Row: row, Column: col,
			Student: opt[studentIdx].Name, StudentID: opt[studentIdx].ID,
			Satisfaction: getSatisfactionDetails(bestIndices, row, col, studentIdx, weights, req.ClassConfig, friends, enemies, opt),
		}
	}
	return response, bestAns
}

func getSatisfactionDetails(seating []int, row, col, studentIndex int, w Weights, config ClassConfig, friends, enemies SocialMap, students []optStudent) SatisfactionDetails {
	var details SatisfactionDetails
	student := students[studentIndex]
	mScore := checkMed(student, row, col)
	pScore := checkPref(student, row, col)
	fScore := checkFriends(studentIndex, seating, row, col, config, friends, len(students))
	ePenalty := checkEnemies(studentIndex, seating, row, col, config, enemies, len(students))
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
		currentGood += (pScore * w.PrefBonus) + (fScore * w.FriendBonus) + (rScore * w.RowBonus)
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

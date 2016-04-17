package main

import (
	"fmt"
	"math"
	"strconv"

	"github.com/fatih/color"
)

type s struct {
	rankedScore int64
	totalHits   int64
	level       int
}

func opCacheData() {
	// get data
	const fetchQuery = `SELECT users.id as user_id, users.username, scores.play_mode, scores.score, scores.completed, scores.300_count, scores.100_count, scores.50_count FROM scores LEFT JOIN users ON users.username=scores.username WHERE users.allowed = '1'`
	rows, err := db.Query(fetchQuery)
	if err != nil {
		queryError(err, fetchQuery)
		return
	}

	// set up end map where all the data is
	data := make(map[int]*[4]*s)

	count := 0

	// analyse every result row of fetchQuery
	for rows.Next() {
		if count%1000 == 0 {
			fmt.Println("> CacheData:", count)
		}
		var (
			uid       int
			username  string
			playMode  int
			score     int64
			completed int
			count300  int
			count100  int
			count50   int
		)
		err := rows.Scan(&uid, &username, &playMode, &score, &completed, &count300, &count100, &count50)
		if err != nil {
			queryError(err, fetchQuery)
			continue
		}
		// silently ignore invalid modes
		if playMode > 3 || playMode < 0 {
			continue
		}
		// create key in map if not already existing
		if _, ex := data[uid]; !ex {
			data[uid] = &[4]*s{}
			for i := 0; i < 4; i++ {
				data[uid][i] = &s{}
			}
		}
		// if the score counts as completed and top score, add it to the ranked score sum
		if c.CacheRankedScore && completed == 3 {
			data[uid][playMode].rankedScore += score
		}
		// add to the number of totalhits count of {300,100,50} hits
		if c.CacheTotalHits {
			data[uid][playMode].totalHits += int64(count300) + int64(count100) + int64(count50)
		}
		count++
	}
	rows.Close()

	if c.CacheLevel {
		const totalScoreQuery = "SELECT id, total_score_std, total_score_taiko, total_score_ctb, total_score_mania FROM users_stats"
		rows, err := db.Query(totalScoreQuery)
		if err != nil {
			queryError(err, totalScoreQuery)
			return
		}
		count = 0
		for rows.Next() {
			if count%100 == 0 {
				fmt.Println("> CacheLevel:", count)
			}
			var (
				id    int
				std   int64
				taiko int64
				ctb   int64
				mania int64
			)
			err := rows.Scan(&id, &std, &taiko, &ctb, &mania)
			if err != nil {
				queryError(err, totalScoreQuery)
				continue
			}
			if _, ex := data[id]; !ex {
				data[id] = &[4]*s{}
				for i := 0; i < 4; i++ {
					data[id][i] = &s{}
				}
			}
			data[id][0].level = getLevel(std)
			data[id][1].level = getLevel(taiko)
			data[id][2].level = getLevel(ctb)
			data[id][3].level = getLevel(mania)
			count++
		}
		rows.Close()
	}
	for k, v := range data {
		if v == nil {
			continue
		}
		for modeInt, modeData := range v {
			if modeData == nil {
				continue
			}
			var setQ string
			var params []interface{}
			if c.CacheRankedScore {
				setQ += "ranked_score_" + modeToString(modeInt) + " = ?"
				params = append(params, (*modeData).rankedScore)
			}
			if c.CacheTotalHits {
				if setQ != "" {
					setQ += ", "
				}
				setQ += "total_hits_" + modeToString(modeInt) + " = ?"
				params = append(params, (*modeData).totalHits)
			}
			if c.CacheLevel {
				if setQ != "" {
					setQ += ", "
				}
				setQ += "level_" + modeToString(modeInt) + " = ?"
				params = append(params, (*modeData).level)
			}
			if setQ != "" {
				params = append(params, k)
				op("UPDATE users_stats SET "+setQ+" WHERE id = ?", params...)
			}
		}
	}
	color.Green("> CacheData: done!")
	wg.Done()
}

func getLevel(rankedScore int64) int {
	for i := 1; i < 8000; i++ {
		lScore := getRequiredScoreForLevel(i)
		if rankedScore < lScore {
			return i
		}
	}
	return 8000
}
func getRequiredScoreForLevel(level int) int64 {
	if level <= 100 {
		if level > 1 {
			return int64(math.Floor(float64(5000)/3*(4*math.Pow(float64(level), 3)-3*math.Pow(float64(level), 2)-float64(level)) + math.Floor(1.25*math.Pow(1.8, float64(level)-60))))
		}
		return 1
	}
	return 26931190829 + 100000000000*int64(level-100)
}

var modes = [...]string{
	"std",
	"taiko",
	"ctb",
	"mania",
}

func modeToString(modeID int) string {
	if modeID < len(modes) {
		return modes[modeID]
	}
	return strconv.Itoa(modeID)
}
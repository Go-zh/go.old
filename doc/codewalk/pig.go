// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"math/rand"
)

/*
const (
	win            = 100 // The winning score in a game of Pig
	gamesPerSeries = 10  // The number of games per series to simulate
)
*/

const (
	win            = 100 // 在一场Pig游戏中获胜的分数
	gamesPerSeries = 10  // 每次连续模拟游戏的数量
)

// A score includes scores accumulated in previous turns for each player,
// as well as the points scored by the current player in this turn.

// 总分 score 包括每个玩家前几轮的得分以及本轮中当前玩家的得分。
type score struct {
	player, opponent, thisTurn int
}

// An action transitions stochastically to a resulting score.

// action 将一个动作随机转换为一个分数。
type action func(current score) (result score, turnIsOver bool)

// roll returns the (result, turnIsOver) outcome of simulating a die roll.
// If the roll value is 1, then thisTurn score is abandoned, and the players'
// roles swap.  Otherwise, the roll value is added to thisTurn.

// roll 返回模拟一次掷骰产生的 (result, turnIsOver)。
// 若掷骰的结果 outcome 为1，那么这一轮 thisTurn 的分数就会被抛弃，
// 然后玩家的角色互换。否则，此次掷骰的值就会计入这一轮 thisTurn 中。
func roll(s score) (score, bool) {
	outcome := rand.Intn(6) + 1 // A random int in [1, 6]
	if outcome == 1 {
		return score{s.opponent, s.player, 0}, true
	}
	return score{s.player, s.opponent, outcome + s.thisTurn}, false
}

// stay returns the (result, turnIsOver) outcome of staying.
// thisTurn score is added to the player's score, and the players' roles swap.

// stay 返回停留时产生的结果 (result, turnIsOver)。
// 这一轮 thisTurn 的分数会计入该玩家的总分中，然后玩家的角色互换。
func stay(s score) (score, bool) {
	return score{s.opponent, s.player + s.thisTurn, 0}, true
}

// A strategy chooses an action for any given score.

// strategy 为任何给定的分数 score 返回一个动作 action
type strategy func(score) action

// stayAtK returns a strategy that rolls until thisTurn is at least k, then stays.

// strategy 返回一个策略，该策略继续掷骰直到这一轮 thisTurn 至少为 k，然后停留。
func stayAtK(k int) strategy {
	return func(s score) action {
		if s.thisTurn >= k {
			return stay
		}
		return roll
	}
}

// play simulates a Pig game and returns the winner (0 or 1).

// play 模拟一场Pig游戏并返回赢家（0或1）。
func play(strategy0, strategy1 strategy) int {
	strategies := []strategy{strategy0, strategy1}
	var s score
	var turnIsOver bool
	// 随机决定谁先玩
	currentPlayer := rand.Intn(2) // Randomly decide who plays first
	for s.player+s.thisTurn < win {
		action := strategies[currentPlayer](s)
		s, turnIsOver = action(s)
		if turnIsOver {
			currentPlayer = (currentPlayer + 1) % 2
		}
	}
	return currentPlayer
}

// roundRobin simulates a series of games between every pair of strategies.

// roundRobin 模拟每一对策略 strategies 之间的一系列游戏。
func roundRobin(strategies []strategy) ([]int, int) {
	wins := make([]int, len(strategies))
	for i := 0; i < len(strategies); i++ {
		for j := i + 1; j < len(strategies); j++ {
			for k := 0; k < gamesPerSeries; k++ {
				winner := play(strategies[i], strategies[j])
				if winner == 0 {
					wins[i]++
				} else {
					wins[j]++
				}
			}
		}
	}
	// 不能自己一个人玩
	gamesPerStrategy := gamesPerSeries * (len(strategies) - 1) // no self play
	return wins, gamesPerStrategy
}

// ratioString takes a list of integer values and returns a string that lists
// each value and its percentage of the sum of all values.
// e.g., ratios(1, 2, 3) = "1/6 (16.7%), 2/6 (33.3%), 3/6 (50.0%)"

// ratioString 接受一个整数值的列表并返回一个字符串，
// 它列出了每一个值以及它对于所有值之和的百分比。
// 例如，ratios(1, 2, 3) = "1/6 (16.7%), 2/6 (33.3%), 3/6 (50.0%)"
func ratioString(vals ...int) string {
	total := 0
	for _, val := range vals {
		total += val
	}
	s := ""
	for _, val := range vals {
		if s != "" {
			s += ", "
		}
		pct := 100 * float64(val) / float64(total)
		s += fmt.Sprintf("%d/%d (%0.1f%%)", val, total, pct)
	}
	return s
}

func main() {
	strategies := make([]strategy, win)
	for k := range strategies {
		strategies[k] = stayAtK(k + 1)
	}
	wins, games := roundRobin(strategies)

	for k := range strategies {
		fmt.Printf("Wins, losses staying at k =% 4d: %s\n",
			k+1, ratioString(wins[k], games-wins[k]))
	}
}

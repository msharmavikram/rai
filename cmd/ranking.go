// +build ece408ProjectMode

package cmd

import (
	//"fmt"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/olekukonko/tablewriter"
	"github.com/pkg/errors"
	"github.com/rai-project/auth/provider"
	"github.com/rai-project/client"
	"github.com/rai-project/config"
	"github.com/rai-project/database/mongodb"
	"github.com/spf13/cobra"
	upper "upper.io/db.v3"
)

var numResults int

const (
	maxResults = 100
)

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// rankingCmd represents the ranking command
var rankingCmd = &cobra.Command{}

func init() {
	if !ece408ProjectMode {
		return
	}
	// rankingCmd represents the ranking command
	rankingCmd = &cobra.Command{
		Use:   "ranking",
		Short: "View anonymous rankings.",
		Long:  `View anonymized convolution performance. Only the fastest result for each team is reported.`,
		RunE: func(cmd *cobra.Command, args []string) error {

			db, err := mongodb.NewDatabase(config.App.Name)
			if err != nil {
				return err
			}
			defer db.Close()

			col, err := client.NewEce408JobResponseBodyCollection(db)
			if err != nil {
				return err
			}
			defer col.Close()

			// Get submissions

			condInferencesExist := upper.Cond{"inferences.0 $exists": "true"}
			cond := upper.And(
				condInferencesExist,
				upper.Cond{
					"is_submission":          true,
					"inferences.correctness": 0.8451},
			)

			var jobs client.Ece408JobResponseBodys
			err = col.Find(cond, 0, 0, &jobs)
			if err != nil {
				return err
			}

			// keep only jobs with non-zero runtimes
			jobs = client.FilterNonZeroTimes(jobs)

			// Sort by fastest
			sort.Sort(client.ByMinOpRuntime(jobs))

			// Keep first instance of every team
			jobs = client.KeepFirstTeam(jobs) // Keep fastest entry for each team

			// show only numResults
			if numResults < 0 {
				numResults = maxResults
			}
			numResults = min(numResults, maxResults)
			numResults = min(numResults, len(jobs))
			//jobs = jobs[0:numResults]

			//for _, j := range jobs {
			//	fmt.Println(j)
			//}

			// Get current user details
			prof, err := provider.New()
			if err != nil {
				return err
			}

			ok, err := prof.Verify()
			if err != nil {
				return err
			}
			if !ok {
				return errors.Errorf("cannot authenticate using the credentials in %v", prof.Options().ProfilePath)
			}

			tname, err := client.FindTeamName(prof.Info().Username)

			if tname == "" {
				println("No team name for " + prof.Info().Username)
				return nil
			}

			// Create table of ranking
			table := tablewriter.NewWriter(os.Stdout)
			table.SetHeader([]string{"You", "Rank", "Anonymized Team", "Fastest Conv (ms)"})

			var x int64
			var currentRank int64
			var currentMinOpRunTime float64

			x = 1
			currentRank = 1
			currentMinOpRunTime = 0

			for _, j := range jobs {
				if currentMinOpRunTime != float64(j.MinOpRuntime())/float64(time.Millisecond) {
					currentMinOpRunTime = float64(j.MinOpRuntime()) / float64(time.Millisecond)
					currentRank = x
				}

				if tname == j.Teamname {
					table.Append([]string{tname + " -->", strconv.FormatInt(currentRank, 10), j.Anonymize().Teamname, strconv.FormatFloat(currentMinOpRunTime, 'f', 3, 64)})
				} else {
					table.Append([]string{"", strconv.FormatInt(currentRank, 10), j.Anonymize().Teamname, strconv.FormatFloat(currentMinOpRunTime, 'f', 3, 64)})
				}
				x++
			}
			table.Render()
			return nil
		},
	}
	rankingCmd.Flags().IntVarP(&numResults, "num-results", "n", 10, "Number of results to show (<"+strconv.Itoa(maxResults)+")")
	RootCmd.AddCommand(rankingCmd)
}

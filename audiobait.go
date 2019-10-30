/*
management-interface - Web based management of Raspberry Pis over WiFi
Copyright (C) 2019, The Cacophony Project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program. If not, see <http://www.gnu.org/licenses/>.
*/

package managementinterface

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/TheCacophonyProject/audiobait/audiofilelibrary"
	"github.com/TheCacophonyProject/audiobait/playlist"
	goconfig "github.com/TheCacophonyProject/go-config"
)

// Return recent log entries from the audiobait process
func getAudiobaitLogEntries() []string {

	logEntries := make([]string, 0)
	out, err := exec.Command("/bin/journalctl", "--no-pager", "-u", "audiobait", "--since", "yesterday", "-n", "100").Output()
	// out, err := exec.Command("/bin/journalctl --no-pager -u audiobait --since yesterday").Output()
	if err != nil {
		log.Println("Could not get audiobait logging info:", err)
		logEntries = append(logEntries, "Could not get audiobait logging info.")
		return logEntries
	}

	lines := strings.Split(string(out), "\n")
	if len(lines) <= 1 {
		// Didn't get any useful output.
		log.Println("Could not get audiobait logging info:", err)
		logEntries = append(logEntries, "Could not get audiobait logging info.")
		return logEntries
	}

	// Separate log entries. The first line contains a sort of header that we don't want.
	for _, line := range lines[1:] {
		logEntries = append(logEntries, line)
	}

	// Show the most recent first.
	reverse(logEntries)

	return logEntries

}

func isAudiobaitRunning() bool {

	if runtime.GOOS == "windows" {
		return false
	}

	// Run pgrep command to try and find the audiobait process.
	out, err := exec.Command("pgrep", "audiobait").Output()
	if err != nil {
		return false
	}
	if out != nil {
		return true
	}
	return false

}

// Return the time part of the time.Time struct as a string.
func extractTimeOfDayAsString(t playlist.TimeOfDay) string {
	return t.Format("15:04PM")
}

// DNB TODO: Do we need this struct?
type scheduleResponse struct {
	Schedule playlist.Schedule
}

func loadScheduleFromDisk(audioDirectory string) (*playlist.Schedule, error) {
	filename := filepath.Join(audioDirectory, scheduleFilename)
	jsonData, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var sr scheduleResponse
	if err = json.Unmarshal(jsonData, &sr); err != nil {
		return nil, err
	}
	return &sr.Schedule, nil
}

// AudiobaitHandlerGen is a wrapper for the AudiobaitHandler function.
func AudiobaitHandlerGen(conf *goconfig.Config) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		AudiobaitHandler(w, r, conf)
	}
}

// AudiobaitHandler shows the status and details of audiobait being played on the device.
func AudiobaitHandler(w http.ResponseWriter, r *http.Request, conf *goconfig.Config) {

	// These next 4 structs are used to put the audiobait data into a format that
	// makes it easy to display
	type soundDisplayInfo struct {
		Sound  string
		Volume int
		Wait   int
	}
	type soundDisplayCombo struct {
		From      string
		Every     int
		Until     string
		SoundInfo []soundDisplayInfo
	}
	type soundDisplaySchedule struct {
		Description   string
		ControlNights int
		PlayNights    int
		StartDay      int
		Combos        []soundDisplayCombo
	}
	type audiobaitResponse struct {
		Running      bool
		LogEntries   []string
		Schedule     soundDisplaySchedule
		ErrorMessage string
	}

	// Creat our response
	resp := audiobaitResponse{
		Running: isAudiobaitRunning(),
	}

	// Get the audiobait schedule from disk.
	audio := goconfig.DefaultAudio()
	if err := conf.Unmarshal(goconfig.AudioKey, &audio); err != nil {
		resp.ErrorMessage = errorMessage(err)
		tmpl.ExecuteTemplate(w, "audiobait.html", resp)
		return
	}

	schedule, err := loadScheduleFromDisk(audio.Dir)
	if err != nil {
		resp.ErrorMessage = errorMessage(err)
		tmpl.ExecuteTemplate(w, "audiobait.html", resp)
		return
	}

	// Put the schedule data into a format which makes rendering on html page easy.
	displaySchedule := soundDisplaySchedule{
		Description:   schedule.Description,
		ControlNights: schedule.ControlNights,
		PlayNights:    schedule.PlayNights,
		StartDay:      schedule.StartDay,
	}

	// The schedule file provides us with audio file IDs.  To get the file names, we
	// need to load them off of disk.
	audioLibraryLoaded := true
	audioLibrary, err := audiofilelibrary.OpenLibrary(audio.Dir, scheduleFilename)
	if err != nil {
		audioLibraryLoaded = false
	}

	for _, combo := range schedule.Combos {
		displayCombo := soundDisplayCombo{
			From:  extractTimeOfDayAsString(combo.From),
			Every: combo.Every / 60,
			Until: extractTimeOfDayAsString(combo.Until),
		}
		for j := 0; j < len(combo.Sounds); j++ {
			displayInfo := soundDisplayInfo{
				Sound:  combo.Sounds[j], // Set to the ID from the schedule intially
				Volume: combo.Volumes[j],
			}
			// Try and get file name off of disk.
			if audioLibraryLoaded {
				ID, err := strconv.Atoi(combo.Sounds[j])
				if err == nil {
					fileName, exists := audioLibrary.GetFileNameOnDisk(ID)
					if exists {
						// We have the file name so display this on the html page.
						displayInfo.Sound = fileName
					}
				}
			}
			if j < len(combo.Sounds)-1 {
				displayInfo.Wait = combo.Waits[j+1] / 60
			}
			displayCombo.SoundInfo = append(displayCombo.SoundInfo, displayInfo)
		}
		displaySchedule.Combos = append(displaySchedule.Combos, displayCombo)
	}
	resp.Schedule = displaySchedule

	// Get the log entries that the audiobait program has output recently.
	resp.LogEntries = getAudiobaitLogEntries()

	tmpl.ExecuteTemplate(w, "audiobait.html", resp)

}

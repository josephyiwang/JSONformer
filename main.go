package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode"
)

func ParseFileToJSON(fileName string) {
	backingArray := make([]map[string]interface{}, 0)

	// Open file
	file, err := os.Open(fileName)
	if err != nil {
		fmt.Println("Error opening file:", err)
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	fileString := ""
	for scanner.Scan() {
		fileString += scanner.Text()
	}
	if err := scanner.Err(); err != nil {
		fmt.Println("Error scanning file:", err)
		return
	}
	fileRunes := []rune(strings.TrimSpace(fileString))

	index := 0
	for index < len(fileRunes) {
		backingObject := make(map[string]interface{}, 0)
		runes, nextIndex, valid := RetrieveBetweenBrackets(fileRunes, 0, '{', '}')
		if !valid {
			fmt.Println("Invalid structure!")
			return
		}

		objects := SplitObjects(runes, index+1)

		for _, objStr := range objects {
			ProcessNextObject(objStr, 0, backingObject)

		}

		backingArray = append(backingArray, backingObject)
		index = nextIndex + 1
	}

	jsonString, err := json.MarshalIndent(backingArray, "", "  ")
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(string(jsonString))
	return
}

// Returns the index of the next non whitespace character in the string.
func SeekNextToken(fileRunes []rune, index int) int {
	for index < len(fileRunes) && unicode.IsSpace(fileRunes[index]) {
		index++
	}
	return index
}

// Returns the index and matching status of the next non whitespace, user-requested character in the string.
func SeekNextTargetedToken(fileRunes []rune, index int, target rune, required bool) (int, bool) {
	for index < len(fileRunes) && unicode.IsSpace(fileRunes[index]) {
		index++
	}
	return index, index < len(fileRunes) && fileRunes[index] == target
}

// Processes the next object in the text structure. The provided index MUST be at the start of the object.
func ProcessNextObject(fileRunes []rune, index int, backingObject map[string]interface{}) {
	// Expect that the cursor is already placed on the object.
	nameRunes, nextIndex, valid := RetrieveString(fileRunes, index)
	if !valid {
		return
	}
	name := string(nameRunes)

	index, valid = SeekNextTargetedToken(fileRunes, nextIndex, ':', true)
	if !valid {
		return
	}
	index++
	index, valid = SeekNextTargetedToken(fileRunes, index, '{', true)
	if !valid {
		return
	}

	objectBody, _, valid := RetrieveBetweenBrackets(fileRunes, index, '{', '}')
	if !valid {
		return
	}

	if name == "" {
		return
	}

	index, valid = SeekNextTargetedToken(objectBody, 0, '"', true)
	if !valid {
		return
	}
	objType, index, valid := RetrieveString(objectBody, index)
	if !valid {
		return
	}

	index, valid = SeekNextTargetedToken(objectBody, index, ':', true)
	if !valid {
		return
	}
	index++

	switch strings.TrimSpace(string(objType)) {
	case "N":
		contentRunes, _, valid := RetrieveString(objectBody, index)
		if !valid {
			return
		}

		result := strToNum(string(contentRunes))
		if result != nil {
			backingObject[name] = result
		}
	case "S":
		contentRunes, _, valid := RetrieveString(objectBody, index)
		if !valid {
			return
		}

		content := strings.TrimSpace(string(contentRunes))
		if content == "" {
			return
		}
		contentAsDate := isRFC3339(content)
		if contentAsDate < 0 {
			backingObject[name] = content
		} else {
			backingObject[name] = contentAsDate
		}
	case "BOOL":
		contentRunes, _, valid := RetrieveString(objectBody, index)
		if !valid {
			return
		}
		content := strings.TrimSpace(string(contentRunes))
		switch content {
		case "1", "t", "T", "TRUE", "true", "True":
			backingObject[name] = true
		case "0", "f", "F", "FALSE", "false", "False":
			backingObject[name] = false
		}
	case "NULL":
		contentRunes, _, valid := RetrieveString(objectBody, index)
		if !valid {
			return
		}
		content := strings.TrimSpace(string(contentRunes))
		switch content {
		case "1", "t", "T", "TRUE", "true", "True":
			backingObject[name] = nil
		}
	case "L":
		listContent := strings.TrimSpace(string(objectBody[index:]))
		listRunes, _, valid := RetrieveBetweenBrackets([]rune(listContent), 0, '[', ']')
		if !valid {
			return
		}

		listObjects := SplitObjects(listRunes, 0)
		backingList := make([]interface{}, 0)

		for _, objStr := range listObjects {

			listItemRunes, _, valid := RetrieveBetweenBrackets(objStr, 0, '{', '}')
			if !valid {
				continue
			}
			listItemType, listItemIndex, valid := RetrieveString(listItemRunes, 0)
			if !valid {
				continue
			}
			listItemIndex, valid = SeekNextTargetedToken(listItemRunes, listItemIndex, ':', true)
			if !valid {
				continue
			}
			listItemIndex++

			switch strings.TrimSpace(string(listItemType)) {
			case "N":
				contentRunes, _, valid := RetrieveString(listItemRunes, listItemIndex+1)
				if !valid {
					continue
				}

				content := strings.TrimSpace(string(contentRunes))

				result := strToNum(content)
				if result != nil {
					backingList = append(backingList, result)
				}
			case "S":
				contentRunes, _, valid := RetrieveString(listItemRunes, listItemIndex)
				if !valid {
					continue
				}

				content := strings.TrimSpace(string(contentRunes))
				if content == "" {
					continue
				}
				contentAsDate := isRFC3339(content)
				if contentAsDate < 0 {
					backingList = append(backingList, content)
				} else {
					backingList = append(backingList, contentAsDate)
				}
			case "BOOL":
				contentRunes, _, valid := RetrieveString(listItemRunes, listItemIndex)
				if !valid {
					return
				}
				content := strings.TrimSpace(string(contentRunes))
				switch content {
				case "1", "t", "T", "TRUE", "true", "True":
					backingList = append(backingList, true)
				case "0", "f", "F", "FALSE", "false", "False":
					backingList = append(backingList, false)
				}
			case "NULL":
				contentRunes, _, valid := RetrieveString(listItemRunes, listItemIndex)
				if !valid {
					return
				}
				content := strings.TrimSpace(string(contentRunes))
				switch content {
				case "1", "t", "T", "TRUE", "true", "True":
					backingList = append(backingList, nil)
				}
			}
		}
		if len(backingList) > 0 {
			backingObject[name] = backingList
		}
	case "M":
		listContent := strings.TrimSpace(string(objectBody[index:]))
		listRunes, _, valid := RetrieveBetweenBrackets([]rune(listContent), 0, '{', '}')
		if !valid {
			return
		}
		mapObjects := SplitObjects(listRunes, 0)
		backingMap := make(map[string]interface{}, 0)

		for _, mapObj := range mapObjects {
			ProcessNextObject(mapObj, 0, backingMap)
		}
		backingObject[name] = backingMap
	}
	return
}

func strToNum(str string) interface{} {
	content := strings.TrimLeft(strings.TrimSpace(str), "0")
	floatNumber, err_1 := strconv.ParseFloat(content, 64)
	intNumber, err_2 := strconv.ParseInt(content, 0, 64)
	if err_1 != nil && err_2 != nil {
		return nil
	} else if err_1 == nil && err_2 != nil {
		return floatNumber
	} else if err_1 != nil && err_2 == nil {
		return intNumber
	} else if floatNumber == float64(intNumber) {
		return intNumber
	} else {
		return floatNumber
	}

}

// Attempts to parse the given string as a RFC3339 datetime
func isRFC3339(str string) int {
	t, err := time.Parse(time.RFC3339, str)
	if err != nil {
		return -1
	}
	// Get the timestamp in seconds since epoch
	return int(t.Unix())
}

// Retrieves the characters between the current index (assumed to be a quotation mark) and the next quotation mark.
func RetrieveString(fileRunes []rune, index int) ([]rune, int, bool) {

	// Expect that the cursor is already placed on the string.
	index, valid := SeekNextTargetedToken(fileRunes, index, '"', true)
	if !valid {
		return []rune{}, 0, false
	}

	index++
	startingPoint := index
	for index < len(fileRunes) && fileRunes[index] != '"' {
		if fileRunes[index] == '\\' {
			index++
		}
		index++
	}
	index++
	return fileRunes[startingPoint : index-1], index, true
}

// Retrieves the characters between the current index (assumed to be a quotation mark) and the next quotation mark.
func RetrieveBetweenBrackets(fileRunes []rune, index int, leftBracket rune, rightBracket rune, duplicatesAllowed ...bool) ([]rune, int, bool) {
	bracketCount := 0

	// Expect that the cursor is already placed on the string.
	index, valid := SeekNextTargetedToken(fileRunes, index, leftBracket, true)
	if !valid {
		return []rune{}, 0, false
	}

	index++
	bracketCount++

	startingPoint := index
	inString := false
	for index < len(fileRunes) && bracketCount > 0 {
		switch fileRunes[index] {
		case '\\':
			index++
		case rightBracket:
			if !inString {
				bracketCount--
			}
		case leftBracket:
			if !inString {
				if len(duplicatesAllowed) > 0 {
					fmt.Println("Incorrect bracket found")
					return []rune{}, 0, false
				}
				bracketCount++
			}
		case '"':
			inString = !inString
		}
		index++
	}
	if bracketCount > 0 {
		fmt.Println("Uneven brackets!")
		return []rune{}, 0, false
	}
	return fileRunes[startingPoint : index-1], index, true
}

func SplitObjects(fileRunes []rune, index int) [][]rune {
	objectList := make([][]rune, 0)

	bracketCount := 0
	inString := false
	leftCursor := index
	for index < len(fileRunes) {
		switch fileRunes[index] {
		case '\\':
			index++
		case '{':
			if !inString {
				bracketCount++
			}
		case '}':
			if !inString {
				bracketCount--
			}
		case '"':
			inString = !inString
		case ',':
			if bracketCount == 0 {
				objectList = append(objectList, fileRunes[leftCursor:index])
				leftCursor = index + 1
			}
		}
		index++
	}
	objectList = append(objectList, fileRunes[leftCursor:index])
	return objectList
}

func main() {

	var file_path string
	file_path = "example.txt"

	ParseFileToJSON(file_path)

}

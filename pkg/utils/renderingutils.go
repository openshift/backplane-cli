package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/olekukonko/tablewriter"
	"golang.org/x/term"
)

// RenderTabbedTable only uses tabs and renders based on terminal width available
// It keeps the first column in it's full length and truncates others
func RenderTabbedTable(headers []string, data [][]string) {
	columnPadding := 2
	maxColumnWidth := calculateOptimalWidthsForColumns(data, columnPadding)

	writer := tabwriter.NewWriter(os.Stdout, 0, 0, columnPadding, ' ', tabwriter.TabIndent)

	// print the headers
	fmt.Fprintf(writer, "%s", strings.Join(headers, "\t"))
	fmt.Fprintln(writer)

	// print the rows
	for _, row := range data {
		fmt.Fprintf(writer, "%s", strings.Join(truncateColumns(row, maxColumnWidth), "\t"))
		fmt.Fprintln(writer)
	}

	writer.Flush()
}

// calculateOptimalWidthsForColumns calculates optimal column width for table rendering.
// It reserves space for the first column and distributes remaining terminal width among other columns.
// Returns a fallback width of 200 if terminal width cannot be determined.
func calculateOptimalWidthsForColumns(data [][]string, columnPadding int) int {
	// detect terminal width
	terminalWidth, _, err := term.GetSize(0)
	if err != nil {
		// if the width cannot be read use a fallback value
		return 200
	}

	maxFirstColumnContentLength := int(0)
	for _, row := range data {
		if maxFirstColumnContentLength < len(row[0]) {
			maxFirstColumnContentLength = len(row[0]) // give the first column as much as it needs
		}
	}
	// take the first column out and distribute the rest of the width uniformly, smaller columns tend to waste space
	maxColumnWidth := ((terminalWidth - maxFirstColumnContentLength - columnPadding) / (len(data[0]) - 1)) - 3 // compensate for ...

	return maxColumnWidth
}

// truncateColumns truncates columns (except the first one) to fit within the specified width.
// If content exceeds maxColumnWidth or contains newlines, it's truncated with "..." suffix.
func truncateColumns(row []string, maxColumnWidth int) []string {
	processedRow := []string{}

	for column, content := range row {
		newLine := strings.Index(content, "\n")
		processedContent := content
		if column > 0 {
			if newLine >= 0 && newLine >= maxColumnWidth {
				processedContent = content[:maxColumnWidth] + "..."
			} else if newLine >= 0 {
				processedContent = content[:newLine] + "..."
			} else if len(content) >= maxColumnWidth {
				processedContent = content[:maxColumnWidth] + "..."
			} else {
				processedContent = content
			}
		}

		processedRow = append(processedRow, processedContent)
	}

	return processedRow
}

// RenderTable renders data in a formatted table using the tablewriter library.
// It displays the headers and data with proper alignment and formatting.
func RenderTable(headers []string, data [][]string) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader(headers)
	table.SetAutoWrapText(true)
	table.SetAutoFormatHeaders(true)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetCenterSeparator("")
	table.SetColumnSeparator("")
	table.SetRowSeparator("")
	table.SetHeaderLine(false)
	table.SetBorder(false)
	table.SetTablePadding("\t") // pad with tabs
	table.SetNoWhiteSpace(true)
	table.AppendBulk(data) // Add Bulk Data
	table.Render()
}

// RenderJSON is an effectual function that renders the reader as JSON
// returns err if render fails
func RenderJSON(reader io.Reader) error {
	body, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	resString, err := json.MarshalIndent(body, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(resString))
	return nil
}

// RenderJSONBytes is an effectual function that renders the reader as JSON
// returns err if render fails
func RenderJSONBytes(i interface{}) error {
	resString, err := json.MarshalIndent(i, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(resString))
	return nil
}

package actions

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// ConfirmAction prompts the user to confirm a destructive action
func ConfirmAction(action *Action) (bool, error) {
	fmt.Printf("\n⚠️  Destructive Action Confirmation Required\n\n")
	fmt.Printf("Action Type: %s\n", action.Type)
	fmt.Printf("Target Layer: %s\n", action.Target.Layer)
	fmt.Printf("Target Entity: %s\n", action.Target.EntityID)
	
	if action.Target.Namespace != "" {
		fmt.Printf("Namespace: %s\n", action.Target.Namespace)
	}
	
	if len(action.Parameters) > 0 {
		fmt.Printf("Parameters:\n")
		for k, v := range action.Parameters {
			fmt.Printf("  %s: %s\n", k, v)
		}
	}
	
	fmt.Printf("\nDo you want to proceed? (yes/no): ")
	
	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false, fmt.Errorf("failed to read confirmation: %w", err)
	}
	
	response = strings.TrimSpace(strings.ToLower(response))
	return response == "yes" || response == "y", nil
}

// DisplayActionResult displays the result of an action execution
func DisplayActionResult(result *ActionResult) {
	fmt.Printf("\n")
	
	if result.Success {
		fmt.Printf("✅ Success: %s\n", result.Message)
	} else {
		fmt.Printf("❌ Failed: %s\n", result.Message)
	}
	
	if result.Output != "" {
		fmt.Printf("\nOutput:\n%s\n", result.Output)
	}
	
	if result.Error != "" {
		fmt.Printf("\nError: %s\n", result.Error)
	}
	
	duration := result.EndTime.Sub(result.StartTime)
	fmt.Printf("\nDuration: %v\n", duration)
}

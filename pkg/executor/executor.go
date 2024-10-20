package executor

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"
)

// ExecRequest defines the input for code execution
type ExecRequest struct {
	Language string
	Code     string
}

// ExecuteInteractiveCode runs the submitted code and supports interactive I/O
func ExecuteInteractiveCode(ctx context.Context, req ExecRequest, input <-chan string, output chan<- string) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Generate a unique container name
	containerName := fmt.Sprintf("code-exec-%d", time.Now().UnixNano())

	// Prepare Docker command based on language
	dockerCmd := prepareDockerCommand(timeoutCtx, containerName, req.Language, req.Code)

	stdin, err := dockerCmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("error creating stdin pipe: %w", err)
	}

	stdout, err := dockerCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("error creating stdout pipe: %w", err)
	}

	stderr, err := dockerCmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("error creating stderr pipe: %w", err)
	}

	// Start the Docker command
	if err := dockerCmd.Start(); err != nil {
		return err
	}

	// Goroutine for handling stdout and stderr
	go func() {
		scanner := bufio.NewScanner(io.MultiReader(stdout, stderr))
		for scanner.Scan() {
			select {
			case output <- scanner.Text() + "\n":
			case <-timeoutCtx.Done():
				return
			}
		}
	}()

	// Goroutine for handling stdin
	go func() {
		for {
			select {
			case inputLine, ok := <-input:
				if !ok {
					return
				}
				_, err := fmt.Fprintln(stdin, inputLine)
				if err != nil {
					output <- "Error writing to stdin: " + err.Error() + "\n"
					return
				}
			case <-timeoutCtx.Done():
				return
			}
		}
	}()

	// Wait for the command to finish or the context to be done
	done := make(chan error, 1)
	go func() {
		done <- dockerCmd.Wait()
	}()

	select {
	case <-timeoutCtx.Done():
		// Force kill the container
		killCmd := exec.Command("docker", "kill", containerName)
		if err := killCmd.Run(); err != nil {
			output <- fmt.Sprintf("Failed to kill container: %v\n", err)
		} else {
			output <- "Execution timed out. Container killed.\n"
		}
		return timeoutCtx.Err()
	case err := <-done:
		if err != nil {
			output <- fmt.Sprintf("Execution error: %v\n", err)
		}
		return err
	}
}

func prepareDockerCommand(ctx context.Context, containerName, language, code string) *exec.Cmd {
	dockerImage := "phantasm/busybox"
	baseCmd := []string{
		"docker", "run", "--rm",
		"--name", containerName,
		"-i", "--cpus=0.5", "-m", "100m",
		dockerImage,
	}

	switch strings.ToLower(language) {
	case "python":
		return exec.CommandContext(ctx, baseCmd[0], append(baseCmd[1:], "python", "-c", code)...)
	case "javascript", "js":
		return exec.CommandContext(ctx, baseCmd[0], append(baseCmd[1:], "node", "-e", code)...)
	case "ruby":
		return exec.CommandContext(ctx, baseCmd[0], append(baseCmd[1:], "ruby", "-e", code)...)
	case "java":
		return exec.CommandContext(ctx, baseCmd[0], append(baseCmd[1:],
			"bash", "-c", fmt.Sprintf(`
				echo '%s' > Main.java &&
				javac Main.java &&
				java Main
			`, code))...)
	case "c":
		return exec.CommandContext(ctx, baseCmd[0], append(baseCmd[1:],
			"bash", "-c", fmt.Sprintf(`
				echo '%s' > main.c &&
				gcc main.c -o main &&
				./main
			`, code))...)
	case "cpp", "c++":
		return exec.CommandContext(ctx, baseCmd[0], append(baseCmd[1:],
			"bash", "-c", fmt.Sprintf(`
				echo '%s' > main.cpp &&
				g++ main.cpp -o main &&
				./main
			`, code))...)
	default:
		return exec.CommandContext(ctx, baseCmd[0], append(baseCmd[1:], language, "-c", code)...)
	}
}

// Copyright (c) Facebook, Inc. and its affiliates.
//
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/firmwareci/contest-client/pkg/download"
	"github.com/firmwareci/contest-client/pkg/env"

	"github.com/firmwareci/contest-client/pkg/transport"
	"github.com/linuxboot/contest/pkg/api"
	"github.com/linuxboot/contest/pkg/config"
	"github.com/linuxboot/contest/pkg/event"
	"github.com/linuxboot/contest/pkg/job"
	"github.com/linuxboot/contest/pkg/types"
)

func run(requestor string, transport transport.Transport, stdout io.Writer) error {
	verb := strings.ToLower(flagSet.Arg(0))
	if verb == "" {
		return fmt.Errorf("Missing verb, see --help")
	}
	var resp interface{}
	var err error
	switch verb {
	case "start":

		var jobDesc []byte
		if flagSet.Arg(1) == "" {
			fmt.Fprintf(os.Stderr, "Reading from stdin...\n")
			jd, err := ioutil.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("failed to read job descriptor: %w", err)
			}
			jobDesc = jd
		} else {
			jd, err := ioutil.ReadFile(flagSet.Arg(1))
			if err != nil {
				return fmt.Errorf("failed to read job descriptor: %w", err)
			}
			jobDesc = jd
		}

		jobDescFormat := config.JobDescFormatJSON
		if *flagYAML {
			jobDescFormat = config.JobDescFormatYAML
		}
		jobDescJSON, err := config.ParseJobDescriptor(jobDesc, jobDescFormat)
		if err != nil {
			return fmt.Errorf("failed to parse job descriptor: %w", err)
		}

		// if the variable is set, download the binary
		// the url is presumed to be from a
		// trusted domain
		unparsedURL, set := os.LookupEnv(env.EnvBinUrl)
		if set == true || unparsedURL != "" {

			binaryPath, err := download.DownloadBinary(unparsedURL)
			if err != nil {
				return fmt.Errorf("Error in Download: %v", err)
			}
			fmt.Printf("%v%v", binaryPath, err)

			jobDescJSON, err = addBinPathToTest(jobDescJSON, binaryPath)
			if err != nil {
				return err
			}

		}

		startResp, err := transport.Start(context.Background(), requestor, string(jobDescJSON))
		if err != nil {
			return err
		}
		resp = startResp

		// handle wait
		if *flagWait && startResp.Data.JobID != 0 {
			// print immediately if wait is used
			buffer := &bytes.Buffer{}
			encoder := json.NewEncoder(buffer)
			encoder.SetEscapeHTML(false)
			encoder.SetIndent("", " ")
			err = encoder.Encode(startResp)
			if err != nil {
				return fmt.Errorf("cannot re-encode api.Respose object: %v", err)
			}
			indentedJSON := buffer.String()
			fmt.Fprintf(stdout, "%s", string(indentedJSON))

			fmt.Fprintf(os.Stderr, "\nWaiting for job to complete...\n")
			resp, err = wait(context.Background(), startResp.Data.JobID, jobWaitPoll, requestor, transport)
			if err != nil {
				return err
			}

			// write the result into it's own file so that firmwareci
			// can more easily parse the results
			buffer.Reset()
			if err := encoder.Encode(resp); err != nil {
				return err
			}

			filename, set := os.LookupEnv(env.EnvLogFile)

			// This should be an error condition down the line.
			// To avoid breakage we accept an unset or invalid logfile and
			// just use the default location.
			if filename == "" || set == false {
				fmt.Fprintf(os.Stderr, "No Logfile specified using default location: %s", env.DefaultLogFile)
				filename = env.DefaultLogFile
			}

			dir, _ := filepath.Split(filename)
			_, err = os.Stat(dir)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					if err := os.MkdirAll(dir, 0o770); err != nil {
						return fmt.Errorf("path to store the logs does not exist, error while creating it: %v", err)
					}
				} else {
					return fmt.Errorf("error retrieving logfile path stat: %v", err)
				}
			}

			// Write the Logfile
			if err := os.WriteFile(filename, buffer.Bytes(), 0o770); err != nil {
				return fmt.Errorf("Could not write to job Log file: %w", err)
			}
		}
	case "stop":
		jobID, err := parseJob(flagSet.Arg(1))
		if err != nil {
			return err
		}
		resp, err = transport.Stop(context.Background(), requestor, types.JobID(jobID))
		if err != nil {
			return err
		}
	case "status":
		jobID, err := parseJob(flagSet.Arg(1))
		if err != nil {
			return err
		}
		resp, err = transport.Status(context.Background(), requestor, jobID)
		if err != nil {
			return err
		}
	case "retry":
		jobID, err := parseJob(flagSet.Arg(1))
		if err != nil {
			return err
		}
		resp, err = transport.Retry(context.Background(), requestor, jobID)
		if err != nil {
			return err
		}
	case "list":
		var states []job.State
		for _, sts := range *flagStates {
			st, err := job.EventNameToJobState(event.Name(sts))
			if err != nil {
				return err
			}
			states = append(states, st)
		}
		resp, err = transport.List(context.Background(), requestor, states, *flagTags)
		if err != nil {
			return err
		}
	case "version":
		resp, err = transport.Version(context.Background(), requestor)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("invalid verb: '%s'", verb)
	}
	buffer := &bytes.Buffer{}
	encoder := json.NewEncoder(buffer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", " ")
	err = encoder.Encode(resp)
	if err != nil {
		return fmt.Errorf("cannot re-encode api.Respose object: %v", err)
	}
	stdout.Write(buffer.Bytes())
	return nil
}

func wait(ctx context.Context, jobID types.JobID, jobWaitPoll time.Duration, requestor string, transport transport.Transport) (*api.StatusResponse, error) {
	// keep polling for status till job is completed, used when -wait is set
	for {
		resp, err := transport.Status(context.Background(), requestor, jobID)
		if errors.Is(err, io.EOF) {
			fmt.Fprintf(os.Stderr, "Encounterd EOF, trying again")
			continue
		}
		if err != nil {
			return nil, err
		}
		if resp.Err != nil {
			return nil, fmt.Errorf("server responded with an error: %s", resp.Err)
		}

		jobState := resp.Data.Status.State

		for _, eventName := range job.JobCompletionEvents {
			if string(jobState) == string(eventName) {
				return resp, nil
			}
		}
		// TODO use  time.Ticker instead of time.Sleep
		time.Sleep(jobWaitPoll)
	}
}

func parseJob(jobIDStr string) (types.JobID, error) {
	if jobIDStr == "" {
		return 0, errors.New("missing job ID")
	}
	var jobID types.JobID
	jobIDint, err := strconv.Atoi(jobIDStr)
	if err != nil {
		return 0, fmt.Errorf("Invalid job ID: %s: %v", jobIDStr, err)
	}
	jobID = types.JobID(jobIDint)
	if jobID <= 0 {
		return 0, fmt.Errorf("Invalid job ID: %s: it must be positive", jobIDStr)
	}
	return jobID, nil
}

func addBinPathToTest(testDescr []byte, binaryPath string) ([]byte, error) {
	s := struct{ BinaryPath string }{binaryPath}

	t, err := template.New("insertbinary").Delims("[[", "]]").Parse(string(testDescr))
	if err != nil {
		return nil, err
	}

	returnBuffer := bytes.Buffer{}

	if err := t.Execute(&returnBuffer, s); err != nil {
		return nil, err
	}

	return returnBuffer.Bytes(), nil
}

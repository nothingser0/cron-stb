package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"text/tabwriter"

	"cron-stb/internal/config"
	"cron-stb/internal/jobs"
	"github.com/robfig/cron/v3"
)

func main() {
	var dataDir string
	var envFile string
	var dryRun bool

	fs := flag.NewFlagSet("cron-stb", flag.ContinueOnError)
	fs.StringVar(&dataDir, "dir", "", "Custom data directory")
	fs.StringVar(&dataDir, "d", "", "Shorthand for -dir")
	fs.StringVar(&envFile, "env", "", "Path to .env file")
	fs.StringVar(&envFile, "e", "", "Shorthand for -env")
	fs.BoolVar(&dryRun, "dry-run", false, "Preview output without sending to Discord")

	if err := fs.Parse(os.Args[1:]); err != nil {
		os.Exit(1)
	}

	config.OverrideDataDir = dataDir
	config.OverrideEnvFile = envFile

	args := fs.Args()
	if len(args) == 0 {
		runDaemon()
		return
	}

	switch args[0] {
	case "daemon":
		runDaemon()
	case "list":
		listJobs()
	case "config", "status":
		config.LoadConfig().PrintStatus()
	case "run":
		if len(args) < 2 {
			fmt.Println("Usage: cron-stb run <job_name>")
			os.Exit(1)
		}
		runSingleJob(args[1], dryRun)
	case "help", "-h", "--help":
		printHelp()
	default:
		fmt.Printf("unknown command: %s\n\n", args[0])
		printHelp()
		os.Exit(1)
	}
}

func printHelp() {
	fmt.Println("Usage: cron-stb [options] [command]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  daemon          Start cron scheduler daemon (default)")
	fmt.Println("  list            List all registered jobs")
	fmt.Println("  run <job_name>  Run a single job manually")
	fmt.Println("  config          Print detected configuration")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  -d, -dir <path> Custom data folder")
	fmt.Println("  -e, -env <path> Custom .env file")
	fmt.Println("  -dry-run        Execute without posting to Discord")
}

func listJobs() {
	all := jobs.GetAllJobs()
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NAME\tSCHEDULE\tDESCRIPTION")
	fmt.Fprintln(w, "----\t--------\t-----------")
	for _, j := range all {
		fmt.Fprintf(w, "%s\t%s\t%s\n", j.Name(), j.Schedule(), j.Description())
	}
	w.Flush()
}

func runSingleJob(name string, dryRun bool) {
	job, ok := jobs.GetJob(name)
	if !ok {
		log.Fatalf("job '%s' not found", name)
	}

	ctx := context.WithValue(context.Background(), jobs.ManualRunKey, true)
	if dryRun {
		ctx = context.WithValue(ctx, jobs.DryRunKey, true)
	}

	if err := job.Run(ctx); err != nil {
		log.Fatalf("job '%s' failed: %v", name, err)
	}
}

func runDaemon() {
	all := jobs.GetAllJobs()
	if len(all) == 0 {
		log.Println("no jobs registered")
		return
	}

	c := cron.New()

	for _, j := range all {
		job := j
		schedule := job.Schedule()
		name := job.Name()

		_, err := c.AddFunc(schedule, func() {
			log.Printf("[cron] starting %s", name)
			if err := job.Run(context.Background()); err != nil {
				log.Printf("[cron] %s error: %v", name, err)
			} else {
				log.Printf("[cron] %s finished", name)
			}
		})

		if err != nil {
			log.Printf("[cron] failed to schedule %s (%s): %v", name, schedule, err)
		} else {
			log.Printf("[cron] registered %-17s [%s]", name, schedule)
		}
	}

	c.Start()
	log.Printf("daemon started with %d jobs", len(all))

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	<-sigChan
	log.Println("stopping daemon...")
	c.Stop()
}

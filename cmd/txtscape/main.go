package main

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
)

type pageData struct {
	Title       string
	Description string
	OG          bool
	Nav         template.HTML
	Body        template.HTML
}

var tutorialSteps = []struct {
	Slug string
	Name string
	Lead string
	Desc string
}{
	{"1", "Install", "Create the project and connect txtscape to your IDE.", "Install txtscape, configure your IDE, and create the Minotaur's Labyrinth project."},
	{"2", "Define", "Design the shape of your world before writing a single room.", "Define a rooms concern with a template, then ask the agent to create your first room. The template shapes the output."},
	{"3", "Populate", "One room is not much of a labyrinth. Let's fill it out.", "Ask your AI agent to create more rooms for the labyrinth. Fill the world with connected locations."},
	{"4", "Deepen", "Rooms are only half a story. Your labyrinth needs inhabitants.", "Add a characters concern and create the Minotaur and Ariadne. Apply the pattern you learned in step 2, on your own."},
	{"5", "Build", "Five rooms. Two characters. Time to make it playable.", "Ask your AI agent to generate a playable text adventure from all the pages in txtscape."},
	{"6", "Revise", "Change a room. Rebuild the game. See the difference.", "Edit a room in txtscape and regenerate the game. The loop closes: change knowledge, rebuild code, see the difference."},
}

var navTutorial = template.HTML(`<a href="/">txtscape</a>`)

var navTutorialStep = template.HTML(`<a href="/">txtscape</a>
        <span class="sep">/</span>
        <a href="/tutorial">tutorial</a>`)

func main() {
	layout := template.Must(template.ParseFiles("content/layout.html"))

	render := func(w http.ResponseWriter, data pageData) {
		var buf bytes.Buffer
		if err := layout.Execute(&buf, data); err != nil {
			log.Printf("template error: %v", err)
			http.Error(w, "Internal Server Error", 500)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(buf.Bytes())
	}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		body, err := os.ReadFile("content/index.html")
		if err != nil {
			http.Error(w, "Internal Server Error", 500)
			return
		}
		render(w, pageData{
			Title:       "txtscape — Persistent project memory for AI agents",
			Description: "Give your AI agent persistent memory that lives in your repo. Plain text pages, committed to git, searchable and structured. An MCP server for any IDE.",
			OG:          true,
			Body:        template.HTML(body),
		})
	})

	mux.HandleFunc("GET /style.css", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "content/style.css")
	})

	mux.HandleFunc("GET /tutorial", func(w http.ResponseWriter, r *http.Request) {
		body, err := os.ReadFile("content/tutorial/index.html")
		if err != nil {
			http.Error(w, "Internal Server Error", 500)
			return
		}
		render(w, pageData{
			Title:       "Tutorial — txtscape",
			Description: "Build a text adventure set in the Minotaur's Labyrinth. Learn how txtscape gives your AI agent persistent, structured memory.",
			Nav:         navTutorial,
			Body:        template.HTML(body),
		})
	})

	mux.HandleFunc("GET /tutorial/{step}", func(w http.ResponseWriter, r *http.Request) {
		slug := r.PathValue("step")
		var idx int
		found := false
		for i, s := range tutorialSteps {
			if s.Slug == slug {
				idx = i
				found = true
				break
			}
		}
		if !found {
			http.NotFound(w, r)
			return
		}

		step := tutorialSteps[idx]
		bodyFile, err := os.ReadFile("content/tutorial/" + slug + ".html")
		if err != nil {
			http.Error(w, "Internal Server Error", 500)
			return
		}

		// Build prev/next nav
		var prev, next string
		if idx > 0 {
			prev = fmt.Sprintf(`<a href="/tutorial/%s">&larr; %s</a>`, tutorialSteps[idx-1].Slug, tutorialSteps[idx-1].Name)
		} else {
			prev = `<a href="/tutorial">&larr; Overview</a>`
		}
		if idx < len(tutorialSteps)-1 {
			next = fmt.Sprintf(`<a href="/tutorial/%s">%s &rarr;</a>`, tutorialSteps[idx+1].Slug, tutorialSteps[idx+1].Name)
		} else {
			next = `<a href="/tutorial">Back to overview</a>`
		}

		// Compose full body: step header + file content + nav links
		var buf bytes.Buffer
		fmt.Fprintf(&buf, `<p class="step-num">Step %d of %d</p>`, idx+1, len(tutorialSteps))
		fmt.Fprintf(&buf, "\n    <h1>%s</h1>", step.Name)
		fmt.Fprintf(&buf, "\n    <p class=\"lead\">%s</p>\n\n", step.Lead)
		buf.Write(bodyFile)
		fmt.Fprintf(&buf, "\n\n    <div class=\"nav-links\">\n        %s\n        %s\n    </div>", prev, next)

		render(w, pageData{
			Title:       step.Name + " — txtscape tutorial",
			Description: step.Desc,
			Nav:         navTutorialStep,
			Body:        template.HTML(buf.String()),
		})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	addr := fmt.Sprintf(":%s", port)
	log.Printf("txtscape listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

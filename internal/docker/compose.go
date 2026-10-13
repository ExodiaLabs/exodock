package docker

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/exodialabs/exodock/internal/core"
	"github.com/exodialabs/exodock/internal/safety"
	"github.com/moby/moby/api/types/container"
	sdk "github.com/moby/moby/client"
)

// Compose projects are observations of canonical labels, never reconstructed file paths.
func composeProjects(containers []container.Summary) []core.Resource {
	groups := map[string][]container.Summary{}
	for _, v := range containers {
		if project := v.Labels[projectLabel]; project != "" {
			groups[project] = append(groups[project], v)
		}
	}
	projects := make([]core.Resource, 0, len(groups))
	for project, items := range groups {
		running := 0
		services := map[string]bool{}
		health := ""
		for _, v := range items {
			if v.State == container.StateRunning {
				running++
			}
			if service := v.Labels[serviceLabel]; service != "" {
				services[service] = true
			}
			h := containerResource(v).Health
			if h == "unhealthy" {
				health = "unhealthy"
			} else if h == "starting" && health != "unhealthy" {
				health = "starting"
			}
		}
		names := make([]string, 0, len(services))
		for service := range services {
			names = append(names, service)
		}
		sort.Strings(names)
		state := fmt.Sprintf("%d/%d running", running, len(items))
		projects = append(projects, cleanResource(core.Resource{ID: project, Name: project, Project: project, Kind: core.Compose, State: state, Health: health, Service: strings.Join(names, ", ")}))
	}
	sort.Slice(projects, func(i, j int) bool {
		if projects[i].Name == projects[j].Name {
			return projects[i].ID < projects[j].ID
		}
		return projects[i].Name < projects[j].Name
	})
	return projects
}
func (c *Client) inspectProject(ctx context.Context, project string) (core.Detail, error) {
	out := core.Detail{Resource: core.Resource{ID: project, Name: project, Project: project, Kind: core.Compose}}
	result, err := c.engine.ContainerList(ctx, sdk.ContainerListOptions{All: true, Filters: make(sdk.Filters).Add("label", projectLabel+"="+project)})
	if err != nil {
		return out, safeError(err)
	}
	// Check labels ourselves too: a misbehaving daemon must not mix unrelated projects.
	items := []container.Summary{}
	for _, v := range result.Items {
		if v.Labels[projectLabel] == project {
			items = append(items, v)
		}
	}
	if len(items) == 0 {
		return out, errors.New("Compose project no longer has containers; refresh the inventory")
	}
	out.Resource = composeProjects(items)[0]
	out.Fields = append(out.Fields, core.Field{Name: "Project", Value: safety.Inline(project)}, core.Field{Name: "Containers", Value: out.Resource.State}, core.Field{Name: "Services", Value: out.Resource.Service})
	sort.Slice(items, func(i, j int) bool {
		a, b := containerResource(items[i]), containerResource(items[j])
		if a.Service != b.Service {
			return a.Service < b.Service
		}
		return a.ID < b.ID
	})
	for _, v := range items {
		r := containerResource(v)
		out.Fields = append(out.Fields, core.Field{Name: "Service " + safety.Inline(r.Service), Value: safety.Inline(fmt.Sprintf("%s (%s): %s", r.Name, r.ID, r.State))})
	}
	out.Fields = append(out.Fields, core.Field{Name: "Source files", Value: "Choose local Compose files explicitly before project actions; label paths are not trusted"})
	return out, nil
}

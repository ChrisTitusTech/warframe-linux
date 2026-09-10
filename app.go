package main

import "github.com/ChrisTitusTech/warframe-linux/internal/services"

// App exposes only the catalog service to the desktop renderer.
type App struct {
	catalog *services.Catalog
}

func NewApp() *App { return &App{catalog: services.NewMockCatalog()} }

func (a *App) ListCatalog() []services.Item { return a.catalog.List() }

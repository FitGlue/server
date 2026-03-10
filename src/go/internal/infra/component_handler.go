package infra

import (
	"context"
	"fmt"
	"log/slog"
)

// ComponentHandler wraps a slog.Handler to prepend [component] to the message.
// When a logger has a "component" attribute set (via .With("component", "name")),
// the handler reads it and prepends [name] to the log message.
type ComponentHandler struct {
	Handler   slog.Handler
	Component string
}

// Enabled implements slog.Handler
func (h *ComponentHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.Handler.Enabled(ctx, level)
}

// WithGroup implements slog.Handler
func (h *ComponentHandler) WithGroup(name string) slog.Handler {
	return &ComponentHandler{
		Handler:   h.Handler.WithGroup(name),
		Component: h.Component,
	}
}

// WithAttrs implements slog.Handler
func (h *ComponentHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newComp := h.Component
	for _, a := range attrs {
		if a.Key == "component" {
			newComp = a.Value.String()
		}
	}
	return &ComponentHandler{
		Handler:   h.Handler.WithAttrs(attrs),
		Component: newComp,
	}
}

// Handle implements slog.Handler
func (h *ComponentHandler) Handle(ctx context.Context, r slog.Record) error {
	comp := h.Component

	// Check if component is overridden in the record attributes
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == "component" {
			comp = a.Value.String()
			return false // stop
		}
		return true
	})

	if comp != "" {
		newMsg := fmt.Sprintf("[%s] %s", comp, r.Message)
		// Create a new record with modified message
		// We use r.Time, r.Level, and r.PC to preserve original metadata
		newRecord := slog.NewRecord(r.Time, r.Level, newMsg, r.PC)

		// Copy attributes from the original record
		r.Attrs(func(a slog.Attr) bool {
			newRecord.AddAttrs(a)
			return true
		})
		r = newRecord
	}

	return h.Handler.Handle(ctx, r)
}

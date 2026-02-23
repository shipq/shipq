package channelcompile

import (
	"fmt"
	"log"
	"strings"

	"github.com/shipq/shipq/codegen"
)

// ValidateChannels checks channel definitions for configuration errors.
// It returns an error if any channel violates the rules:
//   - A channel cannot be both IsPublic and have a RequiredRole
//   - A FromClient dispatch type must have a matching handler function
//   - A channel must have at least one FromClient type (nothing to dispatch)
//   - A channel must have at least one FromServer type (nothing to stream)
//   - A public channel must have a RateLimit configured
//
// It logs warnings (but does not error) if a mid-stream FromClient type
// has no matching handler (scaffolding opportunity).
func ValidateChannels(channels []codegen.SerializedChannelInfo) error {
	var errs []string

	for _, ch := range channels {
		// Error if a channel has both IsPublic == true and RequiredRole != ""
		if ch.IsPublic && ch.RequiredRole != "" {
			errs = append(errs, fmt.Sprintf(
				"channel %q: cannot be both public and require role %q",
				ch.Name, ch.RequiredRole,
			))
		}

		// Error if IsPublic == true and RateLimit == nil
		if ch.IsPublic && ch.RateLimit == nil {
			errs = append(errs, fmt.Sprintf(
				"channel %q: public channels must have a rate limit configured",
				ch.Name,
			))
		}

		// Count FromClient and FromServer types
		var fromClientCount int
		var fromServerCount int
		var dispatchMsg *codegen.SerializedMessageInfo

		for i := range ch.Messages {
			msg := &ch.Messages[i]
			switch msg.Direction {
			case "client_to_server":
				fromClientCount++
				if msg.IsDispatch {
					dispatchMsg = msg
				}
			case "server_to_client":
				fromServerCount++
			}
		}

		// Error if a channel has zero FromClient types (nothing to dispatch)
		if fromClientCount == 0 {
			errs = append(errs, fmt.Sprintf(
				"channel %q: must have at least one FromClient message type",
				ch.Name,
			))
		}

		// Error if a channel has zero FromServer types (nothing to stream)
		if fromServerCount == 0 {
			errs = append(errs, fmt.Sprintf(
				"channel %q: must have at least one FromServer message type",
				ch.Name,
			))
		}

		// Error if a FromClient dispatch type has no matching handler function
		if dispatchMsg != nil && dispatchMsg.HandlerName == "" {
			errs = append(errs, fmt.Sprintf(
				"channel %q: dispatch message type %q has no matching handler function (expected Handle%s)",
				ch.Name, dispatchMsg.TypeName, dispatchMsg.TypeName,
			))
		}

		// Warning (log) if a mid-stream FromClient type has no matching handler
		for _, msg := range ch.Messages {
			if msg.Direction != "client_to_server" {
				continue
			}
			if msg.IsDispatch {
				continue
			}
			if msg.HandlerName == "" {
				log.Printf("WARNING: channel %q: mid-stream FromClient type %q has no matching Handle%s function (scaffolding opportunity)",
					ch.Name, msg.TypeName, msg.TypeName)
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("channel validation failed:\n  %s", strings.Join(errs, "\n  "))
	}

	return nil
}

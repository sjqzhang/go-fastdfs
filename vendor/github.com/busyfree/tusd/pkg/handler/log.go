package handler

import (
	"log"
)

func (h *UnroutedHandler) log(eventName string, details ...string) {
	LogEvent(h.logger, eventName, details...)
}

func LogEvent(logger *log.Logger, eventName string, details ...string) {
	result := make([]byte, 0, 100)

	result = append(result, `event="`...)
	result = append(result, eventName...)
	result = append(result, `" `...)

	for i := 0; i < len(details); i += 2 {
		result = append(result, details[i]...)
		result = append(result, `="`...)
		result = append(result, details[i+1]...)
		result = append(result, `" `...)
	}

	result = append(result, "\n"...)
	logger.Output(2, string(result))
}

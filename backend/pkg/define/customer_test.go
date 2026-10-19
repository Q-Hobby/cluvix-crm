package define_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"erpCluvix/pkg/define"
)

func TestIsValidStatusTransition(t *testing.T) {
	tests := []struct {
		name string
		from int
		to   int
		want bool
	}{
		// Trash (1) — D14: không còn đường vào Lead/Ticket, đích duy nhất là Contact(16)
		{"Trash→Contact", define.CustomerStatusTrash, define.CustomerStatusContact, true},
		{"Trash→Ticket (invalid, D14)", define.CustomerStatusTrash, define.CustomerStatusTicket, false},
		{"Trash→Lead (invalid, D14)", define.CustomerStatusTrash, define.CustomerStatusLead, false},
		{"Trash→Customer (invalid)", define.CustomerStatusTrash, define.CustomerStatusCustomer, false},

		// Ticket (2) — D14: Lead thay bằng Contact
		{"Ticket→Contact", define.CustomerStatusTicket, define.CustomerStatusContact, true},
		{"Ticket→Lead (invalid, D14)", define.CustomerStatusTicket, define.CustomerStatusLead, false},
		{"Ticket→Trash", define.CustomerStatusTicket, define.CustomerStatusTrash, true},
		{"Ticket→Customer (invalid)", define.CustomerStatusTicket, define.CustomerStatusCustomer, false},

		// AppointmentCancel (7) — D14: Lead thay bằng Contact
		{"AppointmentCancel→Contact", define.CustomerStatusAppointmentCancel, define.CustomerStatusContact, true},
		{"AppointmentCancel→Lead (invalid, D14)", define.CustomerStatusAppointmentCancel, define.CustomerStatusLead, false},

		// Lost (9) — D14: Lead thay bằng Contact
		{"Lost→Contact", define.CustomerStatusLost, define.CustomerStatusContact, true},
		{"Lost→Lead (invalid, D14)", define.CustomerStatusLost, define.CustomerStatusLead, false},

		// Lead (4)
		{"Lead→Deposit", define.CustomerStatusLead, define.CustomerStatusDeposit, true},
		{"Lead→Appointment", define.CustomerStatusLead, define.CustomerStatusAppointment, true},
		{"Lead→Lost", define.CustomerStatusLead, define.CustomerStatusLost, true},
		{"Lead→Trash", define.CustomerStatusLead, define.CustomerStatusTrash, true},
		{"Lead→Customer (invalid)", define.CustomerStatusLead, define.CustomerStatusCustomer, false},

		// Customer (12)
		{"Customer→Treatment", define.CustomerStatusCustomer, define.CustomerStatusTreatment, true},
		{"Customer→Complaint", define.CustomerStatusCustomer, define.CustomerStatusComplaint, true},
		{"Customer→Broker", define.CustomerStatusCustomer, define.CustomerStatusBroker, true},
		{"Customer→Lead (invalid)", define.CustomerStatusCustomer, define.CustomerStatusLead, false},
		{"Customer→Trash (invalid)", define.CustomerStatusCustomer, define.CustomerStatusTrash, false},

		// OldSystem (3) — nil transitions = allow all
		{"OldSystem→anything allowed", define.CustomerStatusOldSystem, define.CustomerStatusCustomer, true},

		// Same status always valid
		{"Customer→Customer (same)", define.CustomerStatusCustomer, define.CustomerStatusCustomer, true},
		{"Lead→Lead (same)", define.CustomerStatusLead, define.CustomerStatusLead, true},

		// Appointment (6)
		{"Appointment→Visited", define.CustomerStatusAppointment, define.CustomerStatusVisited, true},
		{"Appointment→AppointmentCancel", define.CustomerStatusAppointment, define.CustomerStatusAppointmentCancel, true},
		{"Appointment→Deposit (invalid)", define.CustomerStatusAppointment, define.CustomerStatusDeposit, false},

		// Visited (8)
		{"Visited→Customer", define.CustomerStatusVisited, define.CustomerStatusCustomer, true},
		{"Visited→PreCustomer", define.CustomerStatusVisited, define.CustomerStatusPreCustomer, true},
		{"Visited→Lead (invalid)", define.CustomerStatusVisited, define.CustomerStatusLead, false},

		// Complaint (13)
		{"Complaint→DoneComplaint", define.CustomerStatusComplaint, define.CustomerStatusDoneComplaint, true},
		{"Complaint→Customer", define.CustomerStatusComplaint, define.CustomerStatusCustomer, true},
		{"Complaint→Lead (invalid)", define.CustomerStatusComplaint, define.CustomerStatusLead, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := define.IsValidStatusTransition(tt.from, tt.to)
			assert.Equal(t, tt.want, got,
				"IsValidStatusTransition(%d→%d)", tt.from, tt.to)
		})
	}
}

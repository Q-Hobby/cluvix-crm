package errs

import "errors"

var ErrCustomerNotFound = errors.New("customer_not_found")
var ErrCustomerWrongTenant = errors.New("customer_wrong_tenant")
var ErrServiceHasTreatment = errors.New("service_has_treatment")
var ErrServiceHasPayment = errors.New("service_has_payment")
var ErrQuantityMustBePositive = errors.New("quantity_must_be_positive")
var ErrQuantityRequired = errors.New("quantity_required")

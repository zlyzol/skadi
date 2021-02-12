package common

import (
	"github.com/pkg/errors"
)

type Accounts struct {
	accounts map[string] Account
}
func NewAccounts() *Accounts {
	ah := Accounts{}
	ah.accounts = make(map[string]Account)
	return &ah
}
// Add - 
func (ah *Accounts) Add(acc Account) error {
	if _, exists := ah.accounts[acc.GetName()]; exists {
		return errors.New("account with this name already added %s" + acc.GetName())
	}
	ah.accounts[acc.GetName()] = acc
	return nil
}
// Exists - 
func (ah *Accounts) Exists(acc Account) bool {
	_, exists := ah.accounts[acc.GetName()]; 
	return exists
}
// Get - 
func (ah *Accounts) Get(name string) (Account, bool) {
	acc, exists := ah.accounts[name]
	return acc, exists
}
package service

// ContractServiceOption -
type ContractServiceOption func(*ContractService)

// WithMaxRetryCountContract -
func WithMaxRetryCountContract(count int) ContractServiceOption {
	return func(cs *ContractService) {
		if count > 0 {
			cs.maxRetryCount = count
		}
	}
}

// WithWorkersCountContract -
func WithWorkersCountContract(count int) ContractServiceOption {
	return func(cs *ContractService) {
		if count > 0 {
			cs.workersCount = count
		}
	}
}

// TokenServiceOption -
type TokenServiceOption func(*TokenService)

// WithMaxRetryCountToken -
func WithMaxRetryCountToken(count int) TokenServiceOption {
	return func(ts *TokenService) {
		if count > 0 {
			ts.maxRetryCount = count
		}
	}
}

// WithWorkersCountToken -
func WithWorkersCountToken(count int) TokenServiceOption {
	return func(ts *TokenService) {
		if count > 0 {
			ts.workersCount = count
		}
	}
}

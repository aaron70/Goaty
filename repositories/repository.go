package repositories

var IgnoreUnknownOptions = true

type SaveOption interface{ saveOption(any) error }
type UpdateOption interface{ updateOption(any) error }
type GetOption interface{ getOption(any) error }
type GetAllOption interface{ getAllOption(any) error }
type DeleteOption interface{ deleteOption(any) error }

type Repository[I comparable, E any] interface {
	Save(I, E, ...SaveOption) (E, error)
	Update(I, E) (E, error)
	Get(I) (E, error)
	GetAll() ([]E, error)
	Delete(I) (E, error)
}

package repositories

type Repository[I comparable, E any] interface {
	Save(I, E) (E, error)
	Update(I, E) (E, error)
	Get(I) (E, error)
	GetAll() ([]E, error)
	Delete(I) (E, error) 
}

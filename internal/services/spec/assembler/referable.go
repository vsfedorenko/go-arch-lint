package assembler

import (
	"github.com/vsfedorenko/go-arch-lint/v2/internal/models/domain"
)

func wrap[T any](ref domain.Reference, list []T) []domain.Referable[T] {
	res := make([]domain.Referable[T], len(list))

	for ind, path := range list {
		res[ind] = domain.NewReferable(path, ref)
	}

	return res
}

func unwrap[T any](refList []domain.Referable[T]) []T {
	res := make([]T, len(refList))

	for ind, r := range refList {
		res[ind] = r.Value
	}

	return res
}

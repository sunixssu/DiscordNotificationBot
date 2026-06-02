include data.env

migrate_up:
	migrate -database $(DATABASE_CONNECTION) -path="db/migrations" up

migrate_up_1:
	migrate -database $(DATABASE_CONNECTION) -path="db/migrations" up 1

migrate_down:
	migrate -database $(DATABASE_CONNECTION) -path="db/migrations" down

migrate_down_1:
	migrate -database $(DATABASE_CONNECTION) -path="db/migrations" down 1
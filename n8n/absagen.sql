SELECT a."userId", u."userName", date
FROM public.stammtisch_abwesenheit a
join public.users u on a."userId" = u."userId"
order by date desc
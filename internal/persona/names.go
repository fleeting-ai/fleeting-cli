package persona

// Gold is hcom’s curated 4-letter CVCV names (aannoo/hcom instance_names.rs).
var Gold = []string{
	"luna", "nova", "nora", "zara", "kira", "mila", "lola", "lara", "sara", "nina", "mira",
	"tara", "sora", "dora", "gina", "lina", "viva", "risa", "mimi", "koko", "lili", "navi",
	"ravi", "rani", "riko", "niko", "mako", "saki", "maki", "nami", "loki", "rori", "lori",
	"mori", "nori", "tori", "gigi", "hana", "hiro", "tomo", "sumi", "vega", "kobe", "rafa",
	"lana", "lena", "dara", "niro", "rosa", "vera", "rina", "mika", "hera", "bela", "beni",
	"bono", "dani", "gabi", "haru", "kato", "keno", "kino", "levi", "lila", "mara", "mina",
	"mona", "remi", "romi", "rumi", "suki", "tina", "vito", "zeno", "kiki", "kimi", "milo",
	"gino", "fifi", "nene", "fido", "lilo", "nara", "miro", "rita", "kuma", "neko", "kana",
	"kiri", "kano", "sana", "miko", "haka", "miso", "taro", "boba", "kava", "soda", "data",
	"beta", "sofa", "mono", "moto", "tiki", "koda", "kali", "gala", "hula", "kula", "puma",
	"zola", "zori", "veto", "vivo", "dino", "nemo", "hero", "zero", "memo", "demo", "polo",
	"solo", "logo", "halo", "sumo", "tofu", "guru", "vino", "diva", "dodo", "silo", "peso",
	"lulu", "pita", "feta", "bobo", "fava", "duma", "beto", "moku", "bozo", "tuna", "lava",
	"hobo", "sake", "bali", "kona", "poke", "soho", "boho", "nano", "zulu", "deli", "rose",
	"dosa", "gobi", "kale", "kilo", "limo", "momo", "sari", "soba", "tapa", "toga", "toto",
	"keto", "midi", "mini", "meme", "tutu", "tuba", "todo", "sage", "vase", "tide", "kite",
	"lime", "vibe", "dune", "maze", "rune", "muse", "dove", "vida", "pogo", "magi", "lira",
	"tipi", "soma", "lobo", "fugu", "naga", "zumi", "reko", "valo", "kazu", "mero", "niru",
	"piko", "hazu", "toku", "veki", "lumo", "melo",
}

func Next(taken map[string]bool) string {
	for _, n := range Gold {
		if taken != nil && taken[n] {
			continue
		}
		return n
	}
	return ""
}

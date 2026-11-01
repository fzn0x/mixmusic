package hct

// Palettes are the tonal palettes of a scheme: three of accents, two of
// neutrals, and one of errors.
type Palettes struct {
	Primary, Secondary, Tertiary, Neutral, NeutralVariant, Error Palette
}

// TonalSpot is the palettes of Material's default scheme, tonal spot, of
// a seed colour: its hue at a moderate chroma for the primary, a calmer
// one for the secondary, the hue turned 60° for the tertiary, and neutrals
// barely tinted with it.
func TonalSpot(seed ARGB) Palettes {
	h := FromARGB(seed).Hue
	return Palettes{
		Primary:        Palette{h, 36},
		Secondary:      Palette{h, 16},
		Tertiary:       Palette{sanitizeDegrees(h + 60), 24},
		Neutral:        Palette{h, 6},
		NeutralVariant: Palette{h, 8},
		Error:          Palette{25, 84},
	}
}

// Scheme is the colour roles of a scheme, light or dark, as Material
// names them.
type Scheme struct {
	Primary, OnPrimary, PrimaryContainer, OnPrimaryContainer         ARGB
	Secondary, OnSecondary, SecondaryContainer, OnSecondaryContainer ARGB
	Tertiary, OnTertiary, TertiaryContainer, OnTertiaryContainer     ARGB
	Error, OnError, ErrorContainer, OnErrorContainer                 ARGB
	// Surface is the ground; the containers rise from it, lowest to
	// highest.
	Surface, SurfaceDim, SurfaceBright                                                                           ARGB
	SurfaceContainerLowest, SurfaceContainerLow, SurfaceContainer, SurfaceContainerHigh, SurfaceContainerHighest ARGB
	OnSurface, OnSurfaceVariant, Outline, OutlineVariant                                                         ARGB
	InverseSurface, InverseOnSurface, InversePrimary                                                             ARGB
}

// Scheme is the roles of the palettes, at the tones of Material's 2021
// colour spec at its standard contrast.
func (p Palettes) Scheme(dark bool) Scheme {
	t := func(light, dk float64) float64 {
		if dark {
			return dk
		}
		return light
	}
	return Scheme{
		Primary:            p.Primary.Tone(t(40, 80)),
		OnPrimary:          p.Primary.Tone(t(100, 20)),
		PrimaryContainer:   p.Primary.Tone(t(90, 30)),
		OnPrimaryContainer: p.Primary.Tone(t(30, 90)),

		Secondary:            p.Secondary.Tone(t(40, 80)),
		OnSecondary:          p.Secondary.Tone(t(100, 20)),
		SecondaryContainer:   p.Secondary.Tone(t(90, 30)),
		OnSecondaryContainer: p.Secondary.Tone(t(30, 90)),

		Tertiary:            p.Tertiary.Tone(t(40, 80)),
		OnTertiary:          p.Tertiary.Tone(t(100, 20)),
		TertiaryContainer:   p.Tertiary.Tone(t(90, 30)),
		OnTertiaryContainer: p.Tertiary.Tone(t(30, 90)),

		Error:            p.Error.Tone(t(40, 80)),
		OnError:          p.Error.Tone(t(100, 20)),
		ErrorContainer:   p.Error.Tone(t(90, 30)),
		OnErrorContainer: p.Error.Tone(t(30, 90)),

		Surface:                 p.Neutral.Tone(t(98, 6)),
		SurfaceDim:              p.Neutral.Tone(t(87, 6)),
		SurfaceBright:           p.Neutral.Tone(t(98, 24)),
		SurfaceContainerLowest:  p.Neutral.Tone(t(100, 4)),
		SurfaceContainerLow:     p.Neutral.Tone(t(96, 10)),
		SurfaceContainer:        p.Neutral.Tone(t(94, 12)),
		SurfaceContainerHigh:    p.Neutral.Tone(t(92, 17)),
		SurfaceContainerHighest: p.Neutral.Tone(t(90, 22)),
		OnSurface:               p.Neutral.Tone(t(10, 90)),
		OnSurfaceVariant:        p.NeutralVariant.Tone(t(30, 80)),
		Outline:                 p.NeutralVariant.Tone(t(50, 60)),
		OutlineVariant:          p.NeutralVariant.Tone(t(80, 30)),
		InverseSurface:          p.Neutral.Tone(t(20, 90)),
		InverseOnSurface:        p.Neutral.Tone(t(95, 20)),
		InversePrimary:          p.Primary.Tone(t(80, 40)),
	}
}

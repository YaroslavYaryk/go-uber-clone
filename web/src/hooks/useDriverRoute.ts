import { useEffect, useState } from 'react'
import { Coordinate } from '../types'

interface DriverRoute {
    route: [number, number][] | null
    durationSeconds: number | null
}

export function useDriverRoute(from: Coordinate, to: Coordinate | null): DriverRoute {
    const [route, setRoute] = useState<[number, number][] | null>(null)
    const [durationSeconds, setDurationSeconds] = useState<number | null>(null)

    useEffect(() => {
        if (!to) {
            setRoute(null)
            setDurationSeconds(null)
            return
        }

        const controller = new AbortController()

        const fetchRoute = async () => {
            try {
                const url = `https://router.project-osrm.org/route/v1/driving/${from.longitude},${from.latitude};${to.longitude},${to.latitude}?overview=full&geometries=geojson`
                const res = await fetch(url, { signal: controller.signal })
                const data = await res.json()

                if (data.routes?.[0]) {
                    const coords: [number, number][] = data.routes[0].geometry.coordinates.map(
                        ([lng, lat]: [number, number]) => [lat, lng]
                    )
                    setRoute(coords)
                    setDurationSeconds(Math.round(data.routes[0].duration))
                }
            } catch (err) {
                if ((err as Error).name !== 'AbortError') {
                    console.error('Failed to fetch driver route:', err)
                }
            }
        }

        fetchRoute()
        return () => controller.abort()
    }, [from.latitude, from.longitude, to?.latitude, to?.longitude])

    return { route, durationSeconds }
}

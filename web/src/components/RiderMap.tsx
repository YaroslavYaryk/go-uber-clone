'use client';

import Image from 'next/image';
import { useRiderStreamConnection } from '../hooks/useRiderStreamConnection';
import { MapContainer, Marker, Popup, Rectangle, TileLayer } from 'react-leaflet'
import L from 'leaflet';
import { getGeohashBounds } from '../utils/geohash';
import { useMemo, useRef, useState } from 'react';
import { useDriverRoute } from '../hooks/useDriverRoute';
import { MapClickHandler } from './MapClickHandler';
import { Button } from './ui/button';
import { RouteFare, RequestRideProps, TripPreview, HTTPTripStartResponse } from "../types";
import { RoutingControl } from "./RoutingControl";
import { Polyline } from 'react-leaflet';
import { API_URL } from '../constants';
import { RiderTripOverview } from './RiderTripOverview';
import { BackendEndpoints, HTTPTripPreviewRequestPayload, HTTPTripPreviewResponse, HTTPTripStartRequestPayload } from '../contracts';
import { useRouter } from 'next/navigation';

const userMarker = new L.Icon({
    iconUrl: "https://www.svgrepo.com/show/535711/user.svg",
    iconSize: [40, 40], // Size of the marker
    iconAnchor: [20, 40], // Anchor point
});

const userDestinationMarker = new L.Icon({
    iconUrl: "https://img.icons8.com/color/48/marker.png",
    iconSize: [40, 40], // Size of the marker
    iconAnchor: [20, 40], // Anchor point
});

const driverMarker = new L.Icon({
    iconUrl: "https://www.svgrepo.com/show/25407/car.svg",
    iconSize: [30, 30],
    iconAnchor: [15, 30],
});

interface RiderMapProps {
    onRouteSelected?: (distance: number) => void;
}

export default function RiderMap({ onRouteSelected }: RiderMapProps) {
    const router = useRouter()
    const isPaymentReturn = typeof window !== 'undefined' &&
        new URLSearchParams(window.location.search).get('payment') === 'success'

    const [trip, setTrip] = useState<TripPreview | null>(() => {
        if (typeof window === 'undefined' || !isPaymentReturn) return null
        const stored = localStorage.getItem('rider_trip')
        return stored ? JSON.parse(stored) as TripPreview : null
    })
    const [selectedCarPackage] = useState<RouteFare | null>(null)
    const [destination, setDestination] = useState<[number, number] | null>(() => {
        if (typeof window === 'undefined' || !isPaymentReturn) return null
        const stored = localStorage.getItem('rider_destination')
        return stored ? JSON.parse(stored) as [number, number] : null
    })
    const mapRef = useRef<L.Map>(null)
    const userID = useMemo(() => {
        const stored = localStorage.getItem('rider_user_id')
        if (stored) return stored
        const id = crypto.randomUUID()
        localStorage.setItem('rider_user_id', id)
        return id
    }, [])
    const debounceTimeoutRef = useRef<NodeJS.Timeout | null>(null);

    const location = {
        latitude: 49.8397,
        longitude: 24.0297,
    };

    const {
        drivers,
        error,
        tripStatus,
        assignedDriver,
        paymentSession,
        resetTripStatus
    } = useRiderStreamConnection(location, userID);

    const { route: driverRoute, durationSeconds: driverEta } = useDriverRoute(
        location,
        assignedDriver?.location ?? null
    );

    console.log(tripStatus)

    const handleMapClick = async (e: L.LeafletMouseEvent) => {
        if (trip?.tripID) {
            return
        }

        if (debounceTimeoutRef.current) {
            clearTimeout(debounceTimeoutRef.current);
        }

        debounceTimeoutRef.current = setTimeout(async () => {
            setDestination([e.latlng.lat, e.latlng.lng])

            const data = await requestRidePreview({
                pickup: [location.latitude, location.longitude],
                destination: [e.latlng.lat, e.latlng.lng],
            })
            console.log(data)

            const parsedRoute = data.route.geometry[0].coordinates
                .map((coord) => [coord.longitude, coord.latitude] as [number, number])

            setTrip({
                tripID: "",
                route: parsedRoute,
                rideFares: data.rideFares,
                distance: data.route.distance,
                duration: data.route.duration,
            })

            // Call onRouteSelected with the route distance
            onRouteSelected?.(data.route.distance)
        }, 500);
    }

    const requestRidePreview = async (props: RequestRideProps): Promise<HTTPTripPreviewResponse> => {
        const { pickup, destination } = props
        const payload = {
            userID: userID,
            pickup: {
                latitude: pickup[0],
                longitude: pickup[1],
            },
            destination: {
                latitude: destination[0],
                longitude: destination[1],
            },
        } as HTTPTripPreviewRequestPayload

        const response = await fetch(`${API_URL}${BackendEndpoints.PREVIEW_TRIP}`, {
            method: 'POST',
            body: JSON.stringify(payload),
        })
        const { data } = await response.json() as { data: HTTPTripPreviewResponse }
        return data
    }

    const handleStartTrip = async (fare: RouteFare) => {
        const payload = {
            rideFareID: fare.id,
            userID: userID,
        } as HTTPTripStartRequestPayload

        if (!fare.id) {
            alert("No Fare ID in the payload")
            return
        }

        const response = await fetch(`${API_URL}${BackendEndpoints.START_TRIP}`, {
            method: 'POST',
            body: JSON.stringify(payload),
        })
        const { data } = await response.json() as { data: HTTPTripStartResponse }

        if (response.ok && trip) {
            const updatedTrip = { ...trip, tripID: data.tripID } as TripPreview
            setTrip(updatedTrip)
            localStorage.setItem('rider_trip', JSON.stringify(updatedTrip))
            localStorage.setItem('rider_destination', JSON.stringify(destination))
        }

        return data
    }

    const handleCancelTrip = async () => {
        if (trip?.tripID) {
            await fetch(`${API_URL}${BackendEndpoints.CANCEL_TRIP}`, {
                method: 'POST',
                body: JSON.stringify({ tripID: trip.tripID, userID }),
            }).catch((err) => console.error('failed to cancel trip:', err))
        }

        setTrip(null)
        setDestination(null)
        resetTripStatus()
        localStorage.removeItem('rider_trip')
        localStorage.removeItem('rider_destination')
        if (new URLSearchParams(window.location.search).get('payment') === 'success') {
            router.push('/')
        }
    }

    if (error) {
        return <div>Error: {error}</div>
    }

    return (
        <div className="relative flex flex-col md:flex-row h-screen">
            <div className={`${destination ? 'flex-[0.7]' : 'flex-1'}`}>
                <MapContainer
                    center={[location.latitude, location.longitude]}
                    zoom={13}
                    style={{ height: '100%', width: '100%' }}
                    ref={mapRef}
                >
                    <TileLayer
                        url="https://{s}.basemaps.cartocdn.com/light_all/{z}/{x}/{y}{r}.png"
                        attribution="&copy; <a href='https://www.openstreetmap.org/copyright'>OpenStreetMap</a> contributors &copy; <a href='https://carto.com/'>CARTO</a>"
                    />
                    <Marker position={[location.latitude, location.longitude]} icon={userMarker} />

                    {/* Render geohash grid cells */}
                    {drivers?.map((driver) => (
                        <Rectangle
                            key={`grid-${driver?.geohash}`}
                            bounds={getGeohashBounds(driver?.geohash) as L.LatLngBoundsExpression}
                            pathOptions={{
                                color: '#3388ff',
                                weight: 1,
                                fillOpacity: 0.1
                            }}
                        >
                            <Popup>Geohash: {driver?.geohash}</Popup>
                        </Rectangle>
                    ))}

                    {/* Render driver markers */}
                    {drivers?.map((driver) => (
                        <Marker
                            key={driver?.id}
                            position={[driver?.location?.latitude, driver?.location?.longitude]}
                            icon={driverMarker}
                        >
                            <Popup>
                                Driver ID: {driver?.id}
                                <br />
                                Geohash: {driver?.geohash}
                                <br />
                                Name: {driver?.name}
                                <br />
                                Car Plate: {driver?.carPlate}
                                <br />
                                <Image
                                    src={driver?.profilePicture}
                                    alt={`${driver?.name}'s profile picture`}
                                    width={100}
                                    height={100}
                                />
                            </Popup>
                        </Marker>
                    ))}
                    {destination && (
                        <Marker position={destination} icon={userDestinationMarker}>
                            <Popup>Destination</Popup>
                        </Marker>
                    )}

                    {selectedCarPackage && (
                        <div className="mt-4 z-[9999] absolute bottom-0 right-0">
                            <Button className="w-full">
                                Request Ride with {selectedCarPackage.packageSlug}
                            </Button>
                        </div>
                    )}
                    {trip && (
                        <RoutingControl route={trip.route} />
                    )}
                    {driverRoute && (
                        <Polyline positions={driverRoute} color="green" dashArray="8" weight={3} />
                    )}
                    <MapClickHandler onClick={handleMapClick} />
                </MapContainer>
            </div>

            <div className="flex-[0.4]">
                <RiderTripOverview
                    trip={trip}
                    assignedDriver={assignedDriver}
                    status={tripStatus}
                    paymentSession={paymentSession}
                    driverEtaSeconds={driverEta}
                    onPackageSelect={handleStartTrip}
                    onCancel={handleCancelTrip}
                />
            </div>
        </div>
    )
}
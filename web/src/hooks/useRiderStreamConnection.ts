import { useEffect, useState } from 'react';
import { WEBSOCKET_URL } from "../constants";
import { Trip } from '../types';
import { Driver, Coordinate } from '../types';
import { PaymentEventSessionCreatedData, TripEvents, ServerWsMessage, isValidWsMessage, BackendEndpoints } from '../contracts';

const ASSIGNED_DRIVER_KEY = 'rider_assigned_driver';

export function useRiderStreamConnection(location: Coordinate, userID: string) {
  const [drivers, setDrivers] = useState<Driver[]>([]);
  const [tripStatus, setTripStatus] = useState<TripEvents | null>(() => {
    if (typeof window === 'undefined') return null;
    return new URLSearchParams(window.location.search).get('payment') === 'success'
      ? TripEvents.PaymentSuccess
      : null;
  });
  const [paymentSession, setPaymentSession] = useState<PaymentEventSessionCreatedData | null>(null);
  const [assignedDriver, setAssignedDriver] = useState<Trip["driver"] | null>(() => {
    if (typeof window === 'undefined') return null;
    if (new URLSearchParams(window.location.search).get('payment') !== 'success') return null;
    const stored = localStorage.getItem(ASSIGNED_DRIVER_KEY);
    return stored ? JSON.parse(stored) as Driver : null;
  });
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!userID) return;

    const ws = new WebSocket(`${WEBSOCKET_URL}${BackendEndpoints.WS_RIDERS}?userID=${userID}`);

    ws.onopen = () => {
      // Send initial location
      if (location) {
        ws.send(JSON.stringify({
          type: TripEvents.DriverLocation,
          data: {
            location,
          }
        }));
      }
    };

    ws.onmessage = (event) => {
      const message = JSON.parse(event.data) as ServerWsMessage;

      if (!message || !isValidWsMessage(message)) {
        setError(`Unknown message type "${message}", allowed types are: ${Object.values(TripEvents).join(', ')}`);
        return;
      }

      switch (message.type) {
        case TripEvents.DriverLocation:
          setDrivers(message.data);
          break;
        case TripEvents.PaymentSessionCreated:
          setPaymentSession(message.data);
          setTripStatus(message.type);
          break;
        case TripEvents.DriverAssigned:
          if (message.data.driver) {
            localStorage.setItem(ASSIGNED_DRIVER_KEY, JSON.stringify(message.data.driver));
          }
          setAssignedDriver(message.data.driver);
          setTripStatus(message.type);
          break;
        case TripEvents.Created:
          setTripStatus(message.type);
          break;
        case TripEvents.NoDriversFound:
          setTripStatus(message.type);
          break;
        case TripEvents.PaymentSuccess:
          setTripStatus(message.type);
          break;
      }
    };

    ws.onclose = () => {
      console.log('WebSocket closed');
    };

    ws.onerror = (event) => {
      setError('WebSocket error occurred');
      console.error('WebSocket error:', event);
    };

    return () => {
      console.log('Closing WebSocket');
      if (ws.readyState === WebSocket.OPEN) {
        ws.close();
      }
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [userID]);

  const resetTripStatus = () => {
    setTripStatus(null);
    setPaymentSession(null);
    localStorage.removeItem(ASSIGNED_DRIVER_KEY);
  }

  return { drivers, assignedDriver, error, tripStatus, paymentSession, resetTripStatus };
}

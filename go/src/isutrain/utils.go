package main

import (
	"fmt"
	"time"
)

func checkAvailableDate(date time.Time) bool {
	jst := time.FixedZone("Asia/Tokyo", 9*60*60)
	t := time.Date(2020, 1, 1, 0, 0, 0, 0, jst)
	t = t.AddDate(0, 0, availableDays)

	return date.Before(t)
}

func getUsableTrainClassList(fromStation Station, toStation Station) []string {
	usable := map[string]string{}

	for key, value := range TrainClassMap {
		usable[key] = value
	}

	if !fromStation.IsStopExpress {
		delete(usable, "express")
	}
	if !fromStation.IsStopSemiExpress {
		delete(usable, "semi_express")
	}
	if !fromStation.IsStopLocal {
		delete(usable, "local")
	}

	if !toStation.IsStopExpress {
		delete(usable, "express")
	}
	if !toStation.IsStopSemiExpress {
		delete(usable, "semi_express")
	}
	if !toStation.IsStopLocal {
		delete(usable, "local")
	}

	ret := []string{}
	for _, v := range usable {
		ret = append(ret, v)
	}

	return ret
}
// seatClass: premium, reserved
// isSmokingSeat: T/F
func (train Train)getAvailableSeatsAll(fromStation Station, toStation Station) ([]Seat, []Seat, []Seat, []Seat, error) {
	var err error

	SeatListCacheMutex.Lock()
	seatListPremiumT := SeatListCache[fmt.Sprintf("%s_%s_%t", train.TrainClass, "premium", true)]
	seatListPremiumF := SeatListCache[fmt.Sprintf("%s_%s_%t", train.TrainClass, "premium", false)]
	seatListReservedT := SeatListCache[fmt.Sprintf("%s_%s_%t", train.TrainClass, "reserved", true)]
	seatListReservedF := SeatListCache[fmt.Sprintf("%s_%s_%t", train.TrainClass, "reserved", false)]
	SeatListCacheMutex.Unlock()

	availableSeatMapPremiumT := map[string]Seat{}
	availableSeatMapPremiumF := map[string]Seat{}
	availableSeatMapReservedT := map[string]Seat{}
	availableSeatMapReservedF := map[string]Seat{}

	for _, seat := range seatListPremiumT {
		availableSeatMapPremiumT[fmt.Sprintf("%d_%d_%s", seat.CarNumber, seat.SeatRow, seat.SeatColumn)] = seat
	}
	for _, seat := range seatListPremiumF {
		availableSeatMapPremiumF[fmt.Sprintf("%d_%d_%s", seat.CarNumber, seat.SeatRow, seat.SeatColumn)] = seat
	}
	for _, seat := range seatListReservedT {
		availableSeatMapReservedT[fmt.Sprintf("%d_%d_%s", seat.CarNumber, seat.SeatRow, seat.SeatColumn)] = seat
	}
	for _, seat := range seatListReservedF {
		availableSeatMapReservedF[fmt.Sprintf("%d_%d_%s", seat.CarNumber, seat.SeatRow, seat.SeatColumn)] = seat
	}
	// すでに取られている予約を取得する
	query := `
	SELECT sr.reservation_id, sr.car_number, sr.seat_row, sr.seat_column
	FROM seat_reservations sr, reservations r, seat_master s, station_master std, station_master sta
	WHERE
		r.reservation_id=sr.reservation_id AND
		s.train_class=r.train_class AND
		s.car_number=sr.car_number AND
		s.seat_column=sr.seat_column AND
		s.seat_row=sr.seat_row AND
		std.name=r.departure AND
		sta.name=r.arrival
	`

	if train.IsNobori {
		query += "AND ((sta.id < ? AND ? <= std.id) OR (sta.id < ? AND ? <= std.id) OR (? < sta.id AND std.id < ?))"
	} else {
		query += "AND ((std.id <= ? AND ? < sta.id) OR (std.id <= ? AND ? < sta.id) OR (sta.id < ? AND ? < std.id))"
	}

	seatReservationList := []SeatReservation{}
	err = dbx.Select(&seatReservationList, query, fromStation.ID, fromStation.ID, toStation.ID, toStation.ID, fromStation.ID, toStation.ID)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	for _, seatReservation := range seatReservationList {
		key := fmt.Sprintf("%d_%d_%s", seatReservation.CarNumber, seatReservation.SeatRow, seatReservation.SeatColumn)
		delete(availableSeatMapPremiumT, key)
		delete(availableSeatMapPremiumF, key)
		delete(availableSeatMapReservedT, key)
		delete(availableSeatMapReservedF, key)
	}

	retPT := []Seat{}
	retPF := []Seat{}
	retRT := []Seat{}
	retRF := []Seat{}
	for _, seat := range availableSeatMapPremiumT {
		retPT = append(retPT, seat)
	}
	for _, seat := range availableSeatMapPremiumF {
		retPF = append(retPT, seat)
	}
	for _, seat := range availableSeatMapReservedT {
		retRT = append(retPT, seat)
	}
	for _, seat := range availableSeatMapReservedF {
		retRF = append(retPT, seat)
	}
	return retPT, retPF, retRT, retRF, nil
}


func (train Train) getAvailableSeats(fromStation Station, toStation Station, seatClass string, isSmokingSeat bool) ([]Seat, error) {
	// 指定種別の空き座席を返す

	var err error

	/*
		// 全ての座席を取得する
		query := "SELECT * FROM seat_master WHERE train_class=? AND seat_class=? AND is_smoking_seat=?"
		seatList := []Seat{}
		err = dbx.Select(&seatList, query, train.TrainClass, seatClass, isSmokingSeat)
		if err != nil {
			return nil, err
		}
	*/

	SeatListCacheMutex.Lock()
	seatList := SeatListCache[fmt.Sprintf("%s_%s_%t", train.TrainClass, seatClass, isSmokingSeat)]
	SeatListCacheMutex.Unlock()
	/*
		conn := pool.Get()
		defer conn.Close()
		data, err := redis.Bytes(conn.Do("GET", fmt.Sprintf("%s_%s_%t", train.TrainClass, seatClass, isSmokingSeat)))
		if err != nil {
			return nil, err
		}
		if data == nil {
			return nil, err
		}
		var seatList []Seat
		if err := json.Unmarshal(data, &seatList); err != nil {
			return nil, err
		}
	*/

	availableSeatMap := map[string]Seat{}
	for _, seat := range seatList {
		availableSeatMap[fmt.Sprintf("%d_%d_%s", seat.CarNumber, seat.SeatRow, seat.SeatColumn)] = seat
	}

	// すでに取られている予約を取得する
	query := `
	SELECT sr.reservation_id, sr.car_number, sr.seat_row, sr.seat_column
	FROM seat_reservations sr, reservations r, seat_master s, station_master std, station_master sta
	WHERE
		r.reservation_id=sr.reservation_id AND
		s.train_class=r.train_class AND
		s.car_number=sr.car_number AND
		s.seat_column=sr.seat_column AND
		s.seat_row=sr.seat_row AND
		std.name=r.departure AND
		sta.name=r.arrival
	`

	if train.IsNobori {
		query += "AND ((sta.id < ? AND ? <= std.id) OR (sta.id < ? AND ? <= std.id) OR (? < sta.id AND std.id < ?))"
	} else {
		query += "AND ((std.id <= ? AND ? < sta.id) OR (std.id <= ? AND ? < sta.id) OR (sta.id < ? AND ? < std.id))"
	}

	seatReservationList := []SeatReservation{}
	err = dbx.Select(&seatReservationList, query, fromStation.ID, fromStation.ID, toStation.ID, toStation.ID, fromStation.ID, toStation.ID)
	if err != nil {
		return nil, err
	}

	for _, seatReservation := range seatReservationList {
		key := fmt.Sprintf("%d_%d_%s", seatReservation.CarNumber, seatReservation.SeatRow, seatReservation.SeatColumn)
		delete(availableSeatMap, key)
	}

	ret := []Seat{}
	for _, seat := range availableSeatMap {
		ret = append(ret, seat)
	}
	return ret, nil
}

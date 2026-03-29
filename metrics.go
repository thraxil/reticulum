package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	resizeOperationsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "reticulum_resize_operations_total",
			Help: "Total number of resize operations.",
		},
		[]string{"status", "size"},
	)

	resizeDurationSeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "reticulum_resize_duration_seconds",
			Help:    "Time spent resizing images.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"status", "size"},
	)

	resizeQueueLengthGauge = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "reticulum_resize_queue_length",
			Help: "Current length of the resize queue.",
		},
	)

	neighborsCountGauge = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "reticulum_neighbors_count",
			Help: "Current number of neighbors in the cluster.",
		},
	)

	neighborFailuresTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "reticulum_neighbor_failures_total",
			Help: "Total number of failed neighbor requests.",
		},
	)

	corruptedImagesTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "reticulum_corrupted_images_total",
			Help: "Total number of corrupted images found.",
		},
	)

	repairedImagesTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "reticulum_repaired_images_total",
			Help: "Total number of images repaired.",
		},
	)

	unrepairableImagesTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "reticulum_unrepairable_images_total",
			Help: "Total number of images that could not be repaired.",
		},
	)

	verifiedImagesTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "reticulum_verified_images_total",
			Help: "Total number of images verified.",
		},
	)

	verifierPassTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "reticulum_verifier_pass_total",
			Help: "Total number of verifier passes completed.",
		},
	)

	rebalanceFailuresTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "reticulum_rebalance_failures_total",
			Help: "Total number of rebalance failures.",
		},
	)

	rebalanceSuccessesTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "reticulum_rebalance_successes_total",
			Help: "Total number of rebalance successes.",
		},
	)

	rebalanceCleanupsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "reticulum_rebalance_cleanups_total",
			Help: "Total number of rebalance cleanups.",
		},
	)

	servedLocallyTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "reticulum_served_locally_total",
			Help: "Total number of images served locally.",
		},
	)

	resizeFailuresTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "reticulum_resize_failures_total",
			Help: "Total number of resize failures.",
		},
	)

	servedScaledTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "reticulum_served_scaled_total",
			Help: "Total number of scaled images served.",
		},
	)

	requestsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "reticulum_requests_total",
			Help: "Total number of requests handled.",
		},
	)

	uptimeSecondsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "reticulum_uptime_seconds_total",
			Help: "Total uptime of the server in seconds.",
		},
	)
)

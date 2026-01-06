import http from 'k6/http';
import { check, sleep, group } from 'k6';
import { Rate, Trend, Counter } from 'k6/metrics';

// Custom metrics
const errorRate = new Rate('errors');
const apiLatency = new Trend('api_latency');
const requestCount = new Counter('requests');

// Configuration
const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

// Test scenarios
export const options = {
    scenarios: {
        // Smoke test - verify system works
        smoke: {
            executor: 'constant-vus',
            vus: 1,
            duration: '30s',
            startTime: '0s',
            tags: { scenario: 'smoke' },
        },
        // Load test - normal load
        load: {
            executor: 'ramping-vus',
            startVUs: 0,
            stages: [
                { duration: '1m', target: 50 },   // Ramp up
                { duration: '3m', target: 50 },   // Stay at 50
                { duration: '1m', target: 100 },  // Ramp to 100
                { duration: '3m', target: 100 },  // Stay at 100
                { duration: '1m', target: 0 },    // Ramp down
            ],
            startTime: '30s',
            tags: { scenario: 'load' },
        },
        // Stress test - find breaking point
        stress: {
            executor: 'ramping-vus',
            startVUs: 0,
            stages: [
                { duration: '2m', target: 200 },
                { duration: '5m', target: 200 },
                { duration: '2m', target: 400 },
                { duration: '5m', target: 400 },
                { duration: '2m', target: 0 },
            ],
            startTime: '10m',
            tags: { scenario: 'stress' },
        },
        // Spike test - sudden load
        spike: {
            executor: 'ramping-vus',
            startVUs: 0,
            stages: [
                { duration: '10s', target: 1000 },
                { duration: '1m', target: 1000 },
                { duration: '10s', target: 0 },
            ],
            startTime: '26m',
            tags: { scenario: 'spike' },
        },
    },
    thresholds: {
        http_req_duration: ['p(95)<500', 'p(99)<1000'],
        http_req_failed: ['rate<0.05'],
        errors: ['rate<0.1'],
    },
};

// Test data
let authToken = null;
let userId = null;

// Helper functions
function getHeaders(includeAuth = false) {
    const headers = {
        'Content-Type': 'application/json',
    };
    if (includeAuth && authToken) {
        headers['Authorization'] = `Bearer ${authToken}`;
    }
    return headers;
}

function handleResponse(response, name) {
    apiLatency.add(response.timings.duration);
    requestCount.add(1);

    const success = response.status >= 200 && response.status < 400;
    errorRate.add(!success);

    if (!success) {
        console.log(`${name} failed: ${response.status} - ${response.body}`);
    }

    return success;
}

// Setup - run once before tests
export function setup() {
    // Register a test user
    const username = `loadtest_${Date.now()}`;
    const email = `${username}@test.com`;
    const password = 'LoadTest123!';

    const registerRes = http.post(
        `${BASE_URL}/api/v1/auth/register`,
        JSON.stringify({
            username: username,
            email: email,
            password: password,
        }),
        { headers: getHeaders() }
    );

    if (registerRes.status === 200 || registerRes.status === 201) {
        const body = JSON.parse(registerRes.body);
        return {
            username: username,
            email: email,
            password: password,
            accessToken: body.access_token,
            userId: body.user ? body.user.id : null,
        };
    }

    console.log('Setup failed, using unauthenticated tests');
    return { username, email, password };
}

// Main test function
export default function(data) {
    // Update auth token from setup data
    if (data && data.accessToken) {
        authToken = data.accessToken;
        userId = data.userId;
    }

    group('Health Check', function() {
        const res = http.get(`${BASE_URL}/health`);
        check(res, {
            'health check status is 200': (r) => r.status === 200,
        });
        handleResponse(res, 'health_check');
    });

    group('Authentication', function() {
        // Login
        if (data && data.username) {
            const loginRes = http.post(
                `${BASE_URL}/api/v1/auth/login`,
                JSON.stringify({
                    username: data.username,
                    password: data.password,
                }),
                { headers: getHeaders() }
            );

            check(loginRes, {
                'login successful': (r) => r.status === 200,
                'login has access_token': (r) => {
                    try {
                        const body = JSON.parse(r.body);
                        authToken = body.access_token;
                        return !!body.access_token;
                    } catch (e) {
                        return false;
                    }
                },
            });
            handleResponse(loginRes, 'login');
        }

        // Get current user
        if (authToken) {
            const meRes = http.get(
                `${BASE_URL}/api/v1/auth/me`,
                { headers: getHeaders(true) }
            );

            check(meRes, {
                'get me status is 200': (r) => r.status === 200,
            });
            handleResponse(meRes, 'get_me');
        }
    });

    sleep(0.5);

    group('Tournaments', function() {
        // List tournaments
        const listRes = http.get(
            `${BASE_URL}/api/v1/tournaments?limit=10`,
            { headers: getHeaders(true) }
        );

        check(listRes, {
            'list tournaments status is 200': (r) => r.status === 200,
        });
        handleResponse(listRes, 'list_tournaments');

        // Get specific tournament (if exists)
        try {
            const tournaments = JSON.parse(listRes.body);
            if (tournaments && tournaments.length > 0) {
                const tournamentId = tournaments[0].id;

                const getRes = http.get(
                    `${BASE_URL}/api/v1/tournaments/${tournamentId}`,
                    { headers: getHeaders(true) }
                );

                check(getRes, {
                    'get tournament status is 200': (r) => r.status === 200,
                });
                handleResponse(getRes, 'get_tournament');

                // Get leaderboard
                const leaderboardRes = http.get(
                    `${BASE_URL}/api/v1/tournaments/${tournamentId}/leaderboard`,
                    { headers: getHeaders(true) }
                );

                check(leaderboardRes, {
                    'get leaderboard status is 200': (r) => r.status === 200,
                });
                handleResponse(leaderboardRes, 'get_leaderboard');
            }
        } catch (e) {
            // Ignore parsing errors
        }
    });

    sleep(0.5);

    group('Matches', function() {
        // Get match statistics
        const statsRes = http.get(
            `${BASE_URL}/api/v1/matches/statistics`,
            { headers: getHeaders(true) }
        );

        check(statsRes, {
            'match statistics returns valid response': (r) => r.status === 200 || r.status === 401,
        });
        handleResponse(statsRes, 'match_statistics');

        // List matches
        const listRes = http.get(
            `${BASE_URL}/api/v1/matches?limit=10`,
            { headers: getHeaders(true) }
        );

        check(listRes, {
            'list matches returns valid response': (r) => r.status === 200 || r.status === 401,
        });
        handleResponse(listRes, 'list_matches');
    });

    sleep(0.3);

    group('Programs', function() {
        // List user programs
        if (authToken) {
            const listRes = http.get(
                `${BASE_URL}/api/v1/programs`,
                { headers: getHeaders(true) }
            );

            check(listRes, {
                'list programs status is 200': (r) => r.status === 200,
            });
            handleResponse(listRes, 'list_programs');
        }
    });

    sleep(0.2);
}

// Teardown - run once after tests
export function teardown(data) {
    if (data && data.accessToken) {
        // Logout
        http.post(
            `${BASE_URL}/api/v1/auth/logout`,
            null,
            { headers: { Authorization: `Bearer ${data.accessToken}` } }
        );
    }
    console.log('Load test completed');
}
